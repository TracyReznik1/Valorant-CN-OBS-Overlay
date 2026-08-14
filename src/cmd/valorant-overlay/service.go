// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"time"
)

type ProbeAPI interface {
	QQLogin() (Credentials, error)
	FetchMainRole(Credentials) (any, error)
	FetchValCard(Credentials, RoleSnapshot) (any, error)
	FetchRecentMatches(Credentials, RoleSnapshot, int, int) (any, error)
}

type OverlayService struct {
	State           *OverlayState
	Store           CredentialStore
	NewProbe        func() (ProbeAPI, error)
	RefreshInterval time.Duration
	Now             func() time.Time
}

func (s *OverlayService) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *OverlayService) ensureLogin(p ProbeAPI) (Credentials, any, RoleSnapshot, error) {
	if cached, ok, err := s.Store.Load(); err == nil && ok {
		if raw, e := p.FetchMainRole(cached); e == nil {
			role := parseRoleSnapshot(raw)
			if role.GameRoleID != "" && role.Scene != "" {
				return cached, raw, role, nil
			}
		}
		_ = s.Store.Delete()
	}
	creds, err := p.QQLogin()
	if err != nil {
		return Credentials{}, nil, RoleSnapshot{}, err
	}
	if err := s.Store.Save(creds); err != nil {
		return Credentials{}, nil, RoleSnapshot{}, fmt.Errorf("保存加密登录态失败: %w", err)
	}
	raw, err := p.FetchMainRole(creds)
	if err != nil {
		return Credentials{}, nil, RoleSnapshot{}, err
	}
	role := parseRoleSnapshot(raw)
	if role.GameRoleID == "" {
		return Credentials{}, nil, RoleSnapshot{}, &ProbeError{"role_info", "未找到国服 PC 角色 game_role_id"}
	}
	if role.Scene == "" {
		return Credentials{}, nil, RoleSnapshot{}, &ProbeError{"role_info", "角色信息缺少 scene"}
	}
	return creds, raw, role, nil
}

func (s *OverlayService) fetchMatchPages(p ProbeAPI, c Credentials, r RoleSnapshot) ([]any, bool, error) {
	combined := []any{}
	today := s.now()
	y, m, d := today.Date()
	start := time.Date(y, m, d, 0, 0, 0, 0, today.Location())
	reachedOld := false
	lastFull := false
	for page := 1; page <= 3; page++ {
		raw, err := p.FetchRecentMatches(c, r, page, 10)
		if err != nil {
			return nil, false, err
		}
		items := candidateMatchList(raw)
		if len(items) == 0 {
			lastFull = false
			break
		}
		lastFull = len(items) >= 10
		for _, item := range items {
			combined = append(combined, item)
			if ts, ok := extractMatchTimestamp(item); ok && time.Unix(int64(ts), 0).In(today.Location()).Before(start) {
				reachedOld = true
			}
		}
		if reachedOld || !lastFull {
			break
		}
	}
	return combined, len(combined) >= 30 && lastFull && !reachedOld, nil
}

func (s *OverlayService) refreshInner() (OverlaySnapshot, error) {
	p, err := s.NewProbe()
	if err != nil {
		return OverlaySnapshot{}, err
	}
	creds, roleRaw, role, err := s.ensureLogin(p)
	if err != nil {
		return OverlaySnapshot{}, err
	}
	card, err := p.FetchValCard(creds, role)
	if err != nil {
		return OverlaySnapshot{}, err
	}
	matches, partial, err := s.fetchMatchPages(p, creds, role)
	if err != nil {
		return OverlaySnapshot{}, err
	}
	latest := findLatestCompetitiveMatch(matches)
	summary := summarizeToday(matches, s.now())
	rr := extractCurrentRR(roleRaw, card)
	if rr == "" {
		rr = "--"
	}
	rank := role.TierText
	if rank == "" {
		rank = "--"
	}
	lastChange := extractRRChange(latest)
	if lastChange == "" {
		lastChange = "--"
	}
	return OverlaySnapshot{Rank: rank, RR: rr, TodayWins: summary.Wins, TodayLosses: summary.Losses, LastResult: normalizedMatchResult(latest), LastRRChange: lastChange, RankIcon: extractRankIcon(card), UpdatedAt: s.now().Format(time.RFC3339), TodayPartial: partial}, nil
}

func (s *OverlayService) RefreshOnce() (OverlaySnapshot, error) {
	snap, err := s.refreshInner()
	if err != nil {
		if s.State != nil {
			s.State.MarkStale(err.Error())
		}
		return OverlaySnapshot{}, err
	}
	s.State.Update(snap)
	return s.State.Get(), nil
}

func (s *OverlayService) Run(stop <-chan struct{}) {
	interval := s.RefreshInterval
	if interval < 5*time.Second {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if _, err := s.RefreshOnce(); err != nil {
				fmt.Printf("⚠️ Overlay 刷新失败：%v\n", err)
			}
		}
	}
}
