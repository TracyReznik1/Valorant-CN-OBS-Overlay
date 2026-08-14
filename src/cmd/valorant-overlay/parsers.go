// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ptuiCallbackRE = regexp.MustCompile(`ptuiCB\('([^']*)','([^']*)','([^']*)','([^']*)','([^']*)'`)

var rrKeys = []string{"league_point", "ranked_rating", "rank_rating", "competitive_point", "rr"}
var rrChangeKeys = []string{"competitive_point_change", "ranked_rating_earned", "rank_rating_change"}

type PTUICallback struct {
	Code        string
	RedirectURL string
	Message     string
}

type RoleSnapshot struct {
	RoleName        string
	TierText        string
	GameRoleID      string
	GameOpenID      string
	Scene           string
	RoleLevel       any
	CompetitiveTier any
	LeaguePoint     any
	Raw             map[string]any
}

type TodaySummary struct{ Wins, Losses, Games int }

func calcPTQRToken(qrsig string) int64 {
	var value int64
	for _, ch := range qrsig {
		value += (value << 5) + int64(ch)
	}
	return value & 2147483647
}

func parsePTUICallback(text string) (PTUICallback, bool) {
	m := ptuiCallbackRE.FindStringSubmatch(text)
	if len(m) < 6 {
		return PTUICallback{}, false
	}
	return PTUICallback{
		Code:        m[1],
		RedirectURL: strings.ReplaceAll(strings.ReplaceAll(m[3], `\/`, `/`), `\x26`, `&`),
		Message:     m[5],
	}, true
}

func urlQueryEscape(s string) string { return url.QueryEscape(s) }

func normalizeEscapedURL(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\/`, `/`), `\x26`, `&`)
}

func parseLooseQuery(raw string) map[string]string {
	out := map[string]string{}
	raw = strings.TrimLeft(strings.ReplaceAll(raw, "#&", "&"), "&")
	q, err := url.ParseQuery(raw)
	if err != nil {
		return out
	}
	for k, vals := range q {
		if len(vals) > 0 {
			out[k] = vals[0]
		}
	}
	return out
}

func extractLoginData(value string) map[string]string {
	merged := map[string]string{}
	queue := []string{normalizeEscapedURL(value)}
	visited := map[string]bool{}
	for len(queue) > 0 {
		candidate := queue[0]
		queue = queue[1:]
		if candidate == "" || visited[candidate] {
			continue
		}
		visited[candidate] = true
		decoded := candidate
		for i := 0; i < 3; i++ {
			next, err := url.PathUnescape(decoded)
			if err != nil || next == decoded {
				break
			}
			decoded = next
		}
		params := map[string]string{}
		if u, err := url.Parse(decoded); err == nil {
			for k, v := range parseLooseQuery(u.RawQuery) {
				params[k] = v
			}
			for k, v := range parseLooseQuery(u.Fragment) {
				params[k] = v
			}
		}
		if len(params) == 0 && (strings.Contains(decoded, "openid=") || strings.Contains(decoded, "access_token=")) {
			for k, v := range parseLooseQuery(decoded) {
				params[k] = v
			}
		}
		for k, v := range params {
			if _, ok := merged[k]; !ok {
				merged[k] = v
			}
		}
		for _, k := range []string{"u1", "url", "redirect_uri", "s_url", "jump_url"} {
			if v := params[k]; v != "" && !visited[v] {
				queue = append(queue, normalizeEscapedURL(v))
			}
		}
	}
	return map[string]string{
		"openid": merged["openid"], "access_token": merged["access_token"],
		"appid": merged["appid"], "pay_token": merged["pay_token"], "key": merged["key"],
	}
}

func stringValue(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	default:
		return fmt.Sprint(v)
	}
}

func mapValue(v any) map[string]any { m, _ := v.(map[string]any); return m }

func anyList(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case []map[string]any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = x[i]
		}
		return out
	default:
		return nil
	}
}

func parseRoleSnapshot(data any) RoleSnapshot {
	var role map[string]any
	var roles []map[string]any
	if m := mapValue(data); m != nil {
		for _, item := range anyList(m["list"]) {
			if mm := mapValue(item); mm != nil {
				roles = append(roles, mm)
			}
		}
		if len(roles) == 0 && (m["role_name"] != nil || m["game_role_id"] != nil || m["tier_text"] != nil) {
			role = m
		}
	} else {
		for _, item := range anyList(data) {
			if mm := mapValue(item); mm != nil {
				roles = append(roles, mm)
			}
		}
	}
	if len(roles) > 0 {
		role = roles[0]
		for _, item := range roles {
			if strings.EqualFold(stringValue(item["gameId"]), "agame") {
				role = item
				break
			}
		}
	}
	if role == nil {
		role = map[string]any{}
	}
	first := func(keys ...string) any {
		for _, k := range keys {
			if v := role[k]; v != nil && stringValue(v) != "" {
				return v
			}
		}
		return nil
	}
	return RoleSnapshot{
		RoleName: stringValue(first("role_name", "name")), TierText: stringValue(first("tier_text", "tier")),
		GameRoleID: stringValue(first("game_role_id", "role_id")), GameOpenID: stringValue(role["game_open_id"]),
		Scene: stringValue(role["scene"]), RoleLevel: first("role_level", "level"), CompetitiveTier: role["competitive_tier"],
		LeaguePoint: role["league_point"], Raw: role,
	}
}

func candidateMatchList(data any) []map[string]any {
	out := []map[string]any{}
	appendList := func(v any) bool {
		items := anyList(v)
		if items == nil {
			return false
		}
		for _, item := range items {
			if m := mapValue(item); m != nil {
				out = append(out, m)
			}
		}
		return true
	}
	if appendList(data) {
		return out
	}
	m := mapValue(data)
	if m == nil {
		return out
	}
	for _, k := range []string{"battle_list", "list", "records", "matches"} {
		if appendList(m[k]) {
			return out
		}
	}
	if nested, ok := m["data"]; ok && nested != nil {
		return candidateMatchList(nested)
	}
	return out
}

func isCompetitiveMatch(match map[string]any) bool {
	enum := strings.ToLower(stringValue(firstMapValue(match, "game_enum", "queue_type")))
	mode := stringValue(firstMapValue(match, "game_mode", "mode_name"))
	if enum == "competitive" || enum == "ranked" {
		return true
	}
	return strings.Contains(mode, "排位") || strings.Contains(mode, "竞技")
}

func firstMapValue(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil && stringValue(v) != "" {
			return v
		}
	}
	return nil
}

func findLatestCompetitiveMatch(data any) map[string]any {
	for _, m := range candidateMatchList(data) {
		if isCompetitiveMatch(m) {
			return m
		}
	}
	return nil
}

func extractFirstValue(v any, keys []string) any {
	switch x := v.(type) {
	case map[string]any:
		for _, k := range keys {
			if val, ok := x[k]; ok && val != nil && stringValue(val) != "" {
				return val
			}
		}
		for _, val := range x {
			if found := extractFirstValue(val, keys); found != nil && stringValue(found) != "" {
				return found
			}
		}
	case []any:
		for _, val := range x {
			if found := extractFirstValue(val, keys); found != nil && stringValue(found) != "" {
				return found
			}
		}
	}
	return nil
}

func displayNumber(v any, signed bool) string {
	if v == nil {
		return ""
	}
	text := strings.TrimSpace(stringValue(v))
	if text == "" {
		return ""
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return text
	}
	var base string
	if f == float64(int64(f)) {
		base = strconv.FormatInt(int64(f), 10)
	} else {
		base = strconv.FormatFloat(f, 'f', -1, 64)
	}
	if signed && f > 0 {
		return "+" + base
	}
	return base
}

func extractRankIcon(source any) string {
	m := mapValue(source)
	if m == nil {
		return ""
	}
	card := mapValue(m["card"])
	if card == nil {
		return ""
	}
	left := mapValue(card["left_data"])
	if left == nil {
		return ""
	}
	return strings.TrimSpace(stringValue(left["image_url"]))
}

func extractMatchTimestamp(match map[string]any) (float64, bool) {
	v := firstMapValue(match, "ts", "match_time", "match_start_time", "match_start")
	if v == nil {
		return 0, false
	}
	f, err := strconv.ParseFloat(stringValue(v), 64)
	if err != nil {
		return 0, false
	}
	if f > 1e10 {
		f /= 1000
	}
	return f, true
}

func normalizedMatchResult(match map[string]any) string {
	if match == nil {
		return "未获取到"
	}
	raw := strings.TrimSpace(stringValue(firstMapValue(match, "result_title", "result")))
	l := strings.ToLower(raw)
	for _, x := range []string{"win", "victory", "胜利", "胜", "获胜", "true", "1"} {
		if l == x {
			return "胜利"
		}
	}
	for _, x := range []string{"lose", "loss", "defeat", "失败", "负", "败", "false", "0"} {
		if l == x {
			return "失败"
		}
	}
	for _, x := range []string{"draw", "tie", "平局", "平"} {
		if l == x {
			return "平局"
		}
	}
	if raw == "" {
		return "未获取到"
	}
	return raw
}

func summarizeToday(matches any, now time.Time) TodaySummary {
	out := TodaySummary{}
	y, m, d := now.Date()
	loc := now.Location()
	start := time.Date(y, m, d, 0, 0, 0, 0, loc)
	end := start.Add(24 * time.Hour)
	for _, match := range candidateMatchList(matches) {
		if !isCompetitiveMatch(match) {
			continue
		}
		ts, ok := extractMatchTimestamp(match)
		if !ok {
			continue
		}
		mt := time.Unix(int64(ts), 0).In(loc)
		if mt.Before(start) || !mt.Before(end) {
			continue
		}
		switch normalizedMatchResult(match) {
		case "胜利":
			out.Wins++
			out.Games++
		case "失败":
			out.Losses++
			out.Games++
		case "平局":
			out.Games++
		}
	}
	return out
}

func extractRRChange(match map[string]any) string {
	if match == nil {
		return ""
	}
	if tier := mapValue(match["competitive_tier"]); tier != nil {
		if v := extractFirstValue(tier, rrChangeKeys); v != nil {
			if r := displayNumber(v, true); r != "" {
				return r
			}
		}
	}
	return displayNumber(extractFirstValue(match, rrChangeKeys), true)
}

var rrTitleRE = regexp.MustCompile(`([0-9]{1,3})\s*分`)
var rrSlashRE = regexp.MustCompile(`([0-9]{1,3})\s*/\s*100`)
var rrTextRE = regexp.MustCompile(`(?i)([0-9]{1,3})\s*(RR|分)`)

func extractCurrentRR(sources ...any) string {
	for _, src := range sources {
		if v := extractFirstValue(src, rrKeys); v != nil {
			if r := displayNumber(v, false); r != "" {
				return r
			}
		}
	}
	for _, src := range sources {
		m := mapValue(src)
		if m == nil {
			continue
		}
		card := mapValue(m["card"])
		if card == nil {
			continue
		}
		left := mapValue(card["left_data"])
		if left == nil {
			continue
		}
		title := stringValue(left["title"])
		if mm := rrTitleRE.FindStringSubmatch(title); len(mm) > 1 {
			n, _ := strconv.Atoi(mm[1])
			if n >= 0 && n <= 100 {
				return strconv.Itoa(n)
			}
		}
	}
	for _, src := range sources {
		d := extractFirstValue(src, []string{"rank_detail", "season_rank_detail"})
		text := stringValue(d)
		if text == "" {
			continue
		}
		for _, re := range []*regexp.Regexp{rrSlashRE, rrTextRE} {
			if mm := re.FindStringSubmatch(text); len(mm) > 1 {
				n, _ := strconv.Atoi(mm[1])
				if n >= 0 && n <= 100 {
					return strconv.Itoa(n)
				}
			}
		}
	}
	return ""
}
