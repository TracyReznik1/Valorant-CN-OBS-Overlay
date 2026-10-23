// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var ptuiCallbackRE = regexp.MustCompile(`ptuiCB\('([^']*)','([^']*)','([^']*)','([^']*)','([^']*)'`)

var rrKeys = []string{"league_point", "ranked_rating", "rank_rating", "competitive_point", "rr"}
var rrChangeKeys = []string{"competitive_point_change", "ranked_rating_earned", "rank_rating_change"}

// chinaLocation pins "today" to the Valorant CN daily reset timezone instead
// of whatever timezone the host machine happens to use.
var chinaLocation = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return loc
	}
	return time.FixedZone("CST", 8*60*60)
}()

type PTUICallback struct {
	Code        string
	RedirectURL string
	Message     string
}

type RoleSnapshot struct {
	RoleName   string
	TierText   string
	GameRoleID string
	Scene      string
	Raw        map[string]any
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
	s = strings.ReplaceAll(s, `\/`, `/`)
	s = strings.ReplaceAll(s, `\x26`, `&`)
	s = strings.ReplaceAll(s, `\u0026`, `&`)
	return strings.ReplaceAll(s, "&amp;", "&")
}

// parseLooseQuery parses auth query strings/fragments without corrupting
// tokens that contain a literal "+". url.ParseQuery treats "+" as a space,
// which is correct for form bodies but not for QQ access tokens in fragments.
func parseLooseQuery(raw string) map[string]string {
	out := map[string]string{}
	raw = strings.TrimLeft(strings.ReplaceAll(raw, "#&", "&"), "&#")
	for _, part := range strings.Split(raw, "&") {
		if part == "" {
			continue
		}
		key, value, _ := strings.Cut(part, "=")
		k, err := url.PathUnescape(key)
		if err != nil {
			continue
		}
		v, err := url.PathUnescape(value)
		if err != nil {
			continue
		}
		if _, exists := out[k]; !exists {
			out[k] = v
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
			if _, ok := merged[k]; !ok && v != "" {
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
		GameRoleID: stringValue(first("game_role_id", "role_id")),
		Scene:      stringValue(role["scene"]), Raw: role,
	}
}

// Numeric suffixes in a card title are RR, not a guessed division: parsing
// "辐能战魂 1250分" must not consume the leading 1 as part of the rank name.
const rankNamePattern = `(?:未定级|未评级|无段位|辐能战魂|无畏战魂|(?:黑铁|青铜|白银|黄金|铂金|钻石|超凡|神话)(?:\s*(?:III|II|I|Ⅲ|Ⅱ|Ⅰ))?)`

var rankNameRE = regexp.MustCompile(`(?i)^` + rankNamePattern + `$`)
var rankTitleRE = regexp.MustCompile(`(?i)^(` + rankNamePattern + `)(?:\s*[0-9]+\s*(?:分|RR))?$`)

func validRankName(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	text = strings.TrimSpace(text)
	if !rankNameRE.MatchString(text) {
		return ""
	}
	return text
}

func extractCurrentRank(role RoleSnapshot, card any) string {
	// tier can be an enum (observed: 255 for this unranked account), not a
	// display name. Never guess its mapping from a number alone.
	for _, value := range []any{role.Raw["tier_text"], role.Raw["tier"], role.TierText} {
		if name := validRankName(value); name != "" {
			return name
		}
	}
	left := mapValue(mapValue(mapValue(card)["card"])["left_data"])
	title, _ := left["title"].(string)
	if match := rankTitleRE.FindStringSubmatch(strings.TrimSpace(title)); len(match) > 1 {
		return validRankName(match[1])
	}
	return ""
}

func isUnranked(rank string) bool {
	return rank == "未定级" || rank == "未评级" || rank == "无段位"
}

// Distinguish a genuine empty list from an unrecognised response, otherwise
// schema changes silently replace cached wins/losses with 0/0.
func parseMatchList(data any) ([]map[string]any, bool) {
	items := anyList(data)
	if items == nil {
		m := mapValue(data)
		for _, k := range []string{"battle_list", "list", "records", "matches"} {
			if items = anyList(m[k]); items != nil {
				break
			}
		}
		if items == nil {
			if nested := m["data"]; nested != nil {
				return parseMatchList(nested)
			}
			return nil, false
		}
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		m := mapValue(item)
		if m == nil {
			return nil, false
		}
		out = append(out, m)
	}
	return out, true
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

func findLatestCompetitiveMatch(matches []map[string]any) map[string]any {
	for _, m := range matches {
		if isCompetitiveMatch(m) {
			return m
		}
	}
	return nil
}

// currentStreak counts consecutive wins/losses from the newest competitive
// match backwards. Non-competitive matches are skipped; a draw or an unknown
// result ends the streak rather than guessing.
func matchesSince(matches []map[string]any, since time.Time) []map[string]any {
	if since.IsZero() {
		return matches
	}
	cutoff := since.Unix()
	out := make([]map[string]any, 0, len(matches))
	for _, match := range matches {
		ts, ok := extractMatchTimestamp(match)
		if ok && int64(ts) >= cutoff {
			out = append(out, match)
		}
	}
	return out
}

func currentStreak(matches []map[string]any) (string, int) {
	streak := ""
	count := 0
	for _, match := range matches {
		if !isCompetitiveMatch(match) {
			continue
		}
		result := normalizedMatchResult(match)
		if result != "胜利" && result != "失败" {
			break
		}
		if streak == "" {
			streak = result
			count = 1
			continue
		}
		if result != streak {
			break
		}
		count++
	}
	switch streak {
	case "胜利":
		return "win", count
	case "失败":
		return "loss", count
	default:
		return "none", 0
	}
}

// RR and RR changes are integer counts. Never pass unrecognised API text
// through to the overlay, or round a fraction/non-finite/overflowing number.
func rrInteger(v any, signed bool) string {
	var text string
	switch x := v.(type) {
	case string:
		text = strings.TrimSpace(x)
	case json.Number:
		text = x.String()
	case int, int32, int64:
		text = stringValue(x)
	case float32:
		return rrInteger(float64(x), signed)
	case float64:
		// JSON numbers are decoded as float64. Keep only exactly representable
		// integers; this is a representation limit, not a game-specific RR cap.
		if math.IsNaN(x) || math.IsInf(x, 0) || math.Trunc(x) != x || math.Abs(x) > 1<<53-1 {
			return ""
		}
		text = strconv.FormatFloat(x, 'f', 0, 64)
	default:
		return ""
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil || (!signed && n < 0) {
		return ""
	}
	if signed && n > 0 {
		return "+" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
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
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	if f > 1e10 {
		f /= 1000
	}
	return f, true
}

func sortMatchesByTimestampDesc(matches []map[string]any) {
	sort.SliceStable(matches, func(i, j int) bool {
		ti, oki := extractMatchTimestamp(matches[i])
		tj, okj := extractMatchTimestamp(matches[j])
		if !oki {
			return false
		}
		if !okj {
			return true
		}
		return ti > tj
	})
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

func summarizeToday(matches []map[string]any, now time.Time) TodaySummary {
	out := TodaySummary{}
	local := now.In(chinaLocation)
	y, m, d := local.Date()
	start := time.Date(y, m, d, 0, 0, 0, 0, local.Location())
	end := start.AddDate(0, 0, 1)
	for _, match := range matches {
		if !isCompetitiveMatch(match) {
			continue
		}
		ts, ok := extractMatchTimestamp(match)
		if !ok {
			continue
		}
		mt := time.Unix(int64(ts), 0).In(local.Location())
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
	for _, scope := range []map[string]any{mapValue(match["competitive_tier"]), match} {
		for _, key := range rrChangeKeys {
			if r := rrInteger(scope[key], true); r != "" {
				return r
			}
		}
	}
	return ""
}

// Consume the full digit sequence and require boundaries: 1250 must never
// become 250 or 50, nor may -18, 18.5, or the denominator of 82/100 become RR.
var rrTextRE = regexp.MustCompile(`(?i)(^|[^0-9A-Za-z_+,./\-])([0-9]+)\s*(RR\b|分)(?:$|[^0-9A-Za-z])`)
var rrSlashRE = regexp.MustCompile(`(^|[^0-9A-Za-z_+,./\-])([0-9]+)\s*/\s*100(?:$|[^0-9A-Za-z_./,])`)

type rrScope struct {
	path string
	data map[string]any
}

type rrText struct {
	path string
	text string
}

func currentRRScopes(src any, index int) []rrScope {
	m := mapValue(src)
	path := fmt.Sprintf("source[%d]", index)
	// Use exactly the same role selection as the service, never another game
	// or another player's rating from the main-role list.
	if anyList(src) != nil || (m != nil && anyList(m["list"]) != nil) {
		m = parseRoleSnapshot(src).Raw
		path += ".selected_role"
	}
	scopes := []rrScope{{path, m}}
	if card := mapValue(m["card"]); card != nil {
		scopes = append(scopes, rrScope{path + ".card", card})
		if left := mapValue(card["left_data"]); left != nil {
			scopes = append(scopes, rrScope{path + ".card.left_data", left})
		}
	}
	return scopes
}

func extractCurrentRR(sources ...any) string {
	rr, _ := extractCurrentRRWithSource(sources...)
	return rr
}

func extractCurrentRRWithSource(sources ...any) (string, string) {
	var scopes []rrScope
	for i, src := range sources {
		scopes = append(scopes, currentRRScopes(src, i)...)
	}
	// Preserve the existing structured-field precedence, but only in known
	// current-player scopes. Invalid values must not block a valid fallback.
	for _, scope := range scopes {
		for _, key := range rrKeys {
			if rr := rrInteger(scope.data[key], false); rr != "" {
				return rr, scope.path + "." + key
			}
		}
	}
	var texts []rrText
	for _, scope := range scopes {
		if strings.HasSuffix(scope.path, ".card.left_data") {
			text, _ := scope.data["title"].(string)
			texts = append(texts, rrText{scope.path + ".title", text})
		}
	}
	for _, scope := range scopes {
		for _, key := range []string{"rank_detail", "season_rank_detail"} {
			text, _ := scope.data[key].(string)
			texts = append(texts, rrText{scope.path + "." + key, text})
		}
	}
	// Explicit point counts precede /100 progress across all sources. There
	// is no evidence that higher ranks need a different named API field.
	for _, re := range []*regexp.Regexp{rrTextRE, rrSlashRE} {
		for _, candidate := range texts {
			for _, mm := range re.FindAllStringSubmatchIndex(candidate.text, -1) {
				// A spaced sign, decimal, or slash is still part of the
				// number's context (notably "82 / 100 RR").
				prefix := strings.TrimSpace(candidate.text[:mm[4]])
				if prefix != "" && strings.ContainsRune("+-./,", rune(prefix[len(prefix)-1])) {
					continue
				}
				rr := rrInteger(candidate.text[mm[4]:mm[5]], false)
				if rr == "" {
					continue
				}
				if re == rrSlashRE {
					n, _ := strconv.ParseInt(rr, 10, 64)
					if n > 100 {
						continue
					}
				}
				return rr, candidate.path
			}
		}
	}
	return "", ""
}
