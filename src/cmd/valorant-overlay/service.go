// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
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
	// SessionStart scopes the win/loss streak to this app run. Zero keeps all
	// fetched matches (used by tests and callers that do not need a session).
	SessionStart time.Time
	// RoleCacheTTL avoids calling get_main_role_raw on every refresh. Zero
	// disables the cache (used by tests that mutate role data between calls).
	RoleCacheTTL time.Duration
	// ReloginCh is consumed by Run when the local status page explicitly asks
	// for a new QR login. Nil disables the feature.
	ReloginCh chan struct{}

	mu              sync.Mutex
	cachedCreds     Credentials
	cachedRole      RoleSnapshot
	roleValidatedAt time.Time
	reloginBusy     atomic.Bool
}

func (s *OverlayService) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *OverlayService) roleCacheValid() bool {
	if s.RoleCacheTTL <= 0 || s.cachedRole.GameRoleID == "" || s.cachedRole.Scene == "" {
		return false
	}
	return s.now().Sub(s.roleValidatedAt) < s.RoleCacheTTL
}

func (s *OverlayService) cacheRole(c Credentials, r RoleSnapshot) {
	s.cachedCreds = c
	s.cachedRole = r
	s.roleValidatedAt = s.now()
}

func (s *OverlayService) clearRoleCache() {
	s.cachedCreds = Credentials{}
	s.cachedRole = RoleSnapshot{}
	s.roleValidatedAt = time.Time{}
}

// ensureLogin distinguishes a rejected session from a transient network or
// schema failure. Only a definitive auth rejection may delete the cached
// credentials; background refreshes must never start an interactive QR login.
func (s *OverlayService) ensureLogin(p ProbeAPI, interactive bool) (Credentials, RoleSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.roleCacheValid() {
		return s.cachedCreds, s.cachedRole, nil
	}

	if cached, ok, err := s.Store.Load(); err == nil && ok {
		raw, fetchErr := p.FetchMainRole(cached)
		if fetchErr == nil {
			role := parseRoleSnapshot(raw)
			if validPCRole(role) {
				s.cacheRole(cached, role)
				return cached, role, nil
			}
			fetchErr = probeErr("role_info", KindSchema, "未找到国服 PC 角色 game_role_id 或 scene")
		}
		if isAuthFailure(fetchErr) {
			if !interactive {
				return Credentials{}, RoleSnapshot{}, probeErr("role_info", KindAuth, "登录已失效，请运行 重新登录.bat")
			}
			_ = s.Store.Delete()
			s.clearRoleCache()
		} else {
			return Credentials{}, RoleSnapshot{}, fetchErr
		}
	}

	if !interactive {
		return Credentials{}, RoleSnapshot{}, probeErr("qq_login", KindLogin, "尚未登录，请先运行程序完成 QQ 扫码")
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
	if !validPCRole(role) {
		return Credentials{}, RoleSnapshot{}, probeErr("role_info", KindSchema, "未找到国服 PC 角色 game_role_id 或 scene")
	}
	s.cacheRole(creds, role)
	return creds, role, nil
}

func validPCRole(role RoleSnapshot) bool {
	if role.GameRoleID == "" || role.Scene == "" {
		return false
	}
	// Some responses also include the game id. Never silently use a mobile
	// role for the PC card/match endpoints.
	if gameID := strings.TrimSpace(stringValue(role.Raw["gameId"])); gameID != "" && !strings.EqualFold(gameID, "agame") {
		return false
	}
	return true
}

func (s *OverlayService) fetchMatchPages(p ProbeAPI, c Credentials, r RoleSnapshot) ([]map[string]any, bool, error) {
	combined := []map[string]any{}
	today := s.now().In(chinaLocation)
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
			return nil, false, probeErr("matches_parse", KindSchema, "战绩列表缺失或无法识别，保留上次数据")
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

func (s *OverlayService) refreshInner(interactive bool) (OverlaySnapshot, error) {
	p, err := s.NewProbe()
	if err != nil {
		return OverlaySnapshot{}, err
	}
	creds, role, err := s.ensureLogin(p, interactive)
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
	// The API is expected to return newest-first, but do not depend on it:
	// "last match", "streak" and today's summary all key off the timestamp.
	sortMatchesByTimestampDesc(matches)
	sessionMatches := matches
	if !s.SessionStart.IsZero() {
		sessionMatches = matchesSince(matches, s.SessionStart)
	}
	latest := findLatestCompetitiveMatch(sessionMatches)
	streakType, streakCount := currentStreak(sessionMatches)
	summary := summarizeToday(matches, s.now())
	rank := extractCurrentRank(role, card)
	rr, rrSource := extractCurrentRRWithSource(role.Raw, card)
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
	return OverlaySnapshot{Rank: rank, RR: rr, RRSource: rrSource, Warning: warning, TodayWins: summary.Wins, TodayLosses: summary.Losses, LastResult: normalizedMatchResult(latest), LastRRChange: lastChange, StreakType: streakType, StreakCount: streakCount, RankIcon: extractRankIcon(card), UpdatedAt: s.now().Format(time.RFC3339), TodayPartial: partial}, nil
}

func (s *OverlayService) refreshOnce(interactive bool) (OverlaySnapshot, error) {
	snap, err := s.refreshInner(interactive)
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
	if s.State == nil {
		return snap, nil
	}
	s.State.Update(snap)
	return s.State.Get(), nil
}

// RefreshOnce is the interactive entry point used at startup. It may open the
// QQ QR code when no valid session exists.
func (s *OverlayService) RefreshOnce() (OverlaySnapshot, error) {
	return s.refreshOnce(true)
}

// Start performs the first refresh. Transient network failures keep the
// already-running HTTP server alive and retry with backoff; authentication and
// schema failures are reported immediately so the user can act.
func (s *OverlayService) Start(stop <-chan struct{}) error {
	const warnAfter = 5
	backoff := 2 * time.Second
	reloginTried := false
	for attempt := 1; ; attempt++ {
		_, err := s.refreshOnce(true)
		if err == nil {
			return nil
		}
		fmt.Printf("⚠️ 启动刷新失败（第 %d 次）：%v\n", attempt, err)
		if isAuthFailure(err) && !reloginTried {
			reloginTried = true
			fmt.Println("登录态已失效，准备重新扫码…")
			_ = s.Store.Delete()
			s.clearRoleCache()
			continue
		}
		if isQRRefreshable(err) {
			fmt.Println("二维码已过期或等待超时，正在生成新的二维码…")
			select {
			case <-time.After(1 * time.Second):
			case <-stop:
				return err
			}
			backoff = 2 * time.Second
			continue
		}
		if !isRetryable(err) {
			return err
		}
		if attempt == warnAfter {
			fmt.Println("⚠️ 网络持续不可用：本地服务会保持运行并继续重试，按 Ctrl+C 可退出。")
		}
		select {
		case <-time.After(backoff):
		case <-stop:
			return err
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

// RequestRelogin asks Run to clear the session and start one interactive QR
// login. It never blocks; a pending request or disabled channel returns false.
func (s *OverlayService) RequestRelogin() bool {
	if s.ReloginCh == nil {
		return false
	}
	if !s.reloginBusy.CompareAndSwap(false, true) {
		return false
	}
	select {
	case s.ReloginCh <- struct{}{}:
		return true
	default:
		s.reloginBusy.Store(false)
		return false
	}
}

func (s *OverlayService) Run(stop <-chan struct{}) {
	interval := s.RefreshInterval
	if interval < 5*time.Second {
		interval = 30 * time.Second
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	authWarned := false
	for {
		select {
		case <-stop:
			return
		case <-s.ReloginCh:
			fmt.Println("收到重新登录请求，正在清除登录态…")
			_ = s.Store.Delete()
			s.clearRoleCache()
			if _, err := s.refreshOnce(true); err != nil {
				fmt.Printf("⚠️ 重新登录失败：%v\n", err)
			} else {
				authWarned = false
				fmt.Println("重新登录完成。")
			}
			s.reloginBusy.Store(false)
			timer.Reset(interval)
		case <-timer.C:
			if _, err := s.refreshOnce(false); err != nil {
				if isAuthFailure(err) {
					if !authWarned {
						fmt.Println("⚠️ 登录已失效，请关闭程序后运行 重新登录.bat")
						authWarned = true
					}
				} else {
					authWarned = false
					fmt.Printf("⚠️ Overlay 刷新失败：%v\n", err)
				}
			} else {
				authWarned = false
			}
			timer.Reset(interval)
		}
	}
}

func isAuthFailure(err error) bool {
	var pe *ProbeError
	return errors.As(err, &pe) && pe.Kind == KindAuth
}

func isQRRefreshable(err error) bool {
	var pe *ProbeError
	if !errors.As(err, &pe) {
		return false
	}
	return pe.Stage == "qq_qr_expired" || pe.Stage == "qq_login_timeout"
}

func isRetryable(err error) bool {
	var pe *ProbeError
	return errors.As(err, &pe) && pe.Kind == KindTransient
}
