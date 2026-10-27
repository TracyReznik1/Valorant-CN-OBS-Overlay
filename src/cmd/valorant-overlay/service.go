// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"errors"
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

func (s *OverlayService) ensureLogin(p ProbeAPI) (Credentials, RoleSnapshot, error) {
	if cached, ok, err := s.Store.Load(); err == nil && ok {
		if raw, e := p.FetchMainRole(cached); e == nil {
			role := parseRoleSnapshot(raw)
			if role.GameRoleID != "" && role.Scene != "" {
				return cached, role, nil
			}
		}
		_ = s.Store.Delete()
	}
	creds, err := p.QQLogin()
	if err != nil {
		return Credentials{}, RoleSnapshot{}, err
	}
	if err := s.Store.Save(creds); err != nil {
		return Credentials{}, RoleSnapshot{}, fmt.Errorf("保存加密登录态失败: %w", err)
	}
	raw, err := p.FetchMainRole(creds)
	if err != nil {
		return Credentials{}, RoleSnapshot{}, err
	}
	role := parseRoleSnapshot(raw)
	if role.GameRoleID == "" {
		return Credentials{}, RoleSnapshot{}, &ProbeError{"role_info", "未找到国服 PC 角色 game_role_id"}
	}
	if role.Scene == "" {
		return Credentials{}, RoleSnapshot{}, &ProbeError{"role_info", "角色信息缺少 scene"}
	}
	return creds, role, nil
}

func (s *OverlayService) fetchMatchPages(p ProbeAPI, c Credentials, r RoleSnapshot) ([]map[string]any, bool, error) {
	combined := []map[string]any{}
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
		items, ok := parseMatchList(raw)
		if !ok {
			return nil, false, &ProbeError{"matches_parse", "战绩列表缺失或无法识别，保留上次数据"}
		}
		if len(items) == 0 {
			lastFull = false
			break
		}
		lastFull = len(items) >= 10
		combined = append(combined, items...)
		for _, item := range items {
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
	creds, role, err := s.ensureLogin(p)
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
	rank := extractCurrentRank(role, card)
	rr, rrSource := extractCardFirstRRWithSource(role, card)
	warning := ""
	if rr == "" {
		rr = "--"
		if !isUnranked(rank) {
			warning = "[rr_parse] 当前 RR 字段缺失或无法识别，等待有效数据"
		}
	}
	if rank == "" {
		rank = "--"
		if warning != "" {
			warning += "；"
		}
		warning += "[rank_parse] 当前段位名称缺失或无法识别，等待有效数据"
	}
	lastChange := extractRRChange(latest)
	if lastChange == "" {
		lastChange = "--"
	}
	return OverlaySnapshot{Rank: rank, RR: rr, RRSource: rrSource, Warning: warning, TodayWins: summary.Wins, TodayLosses: summary.Losses, LastResult: normalizedMatchResult(latest), LastRRChange: lastChange, RankIcon: extractRankIcon(card), UpdatedAt: s.now().Format(time.RFC3339), TodayPartial: partial}, nil
}

func (s *OverlayService) RefreshOnce() (OverlaySnapshot, error) {
	snap, err := s.refreshInner()
	if err != nil {
		if s.State != nil {
			message := err.Error()
			var pe *ProbeError
			if errors.As(err, &pe) {
				message = fmt.Sprintf("[%s] %s", pe.Stage, pe.Message)
			}
			s.State.MarkStale(message)
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
