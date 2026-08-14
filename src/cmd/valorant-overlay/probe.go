// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	crand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIBase      = "https://app.mval.qq.com"
	defaultMVALLoginURL = "https://app.mval.qq.com/go/auth/login_by_qq?source_game_zone=agame&game_zone=agame"
	ptqrShowURL         = "https://xui.ptlogin2.qq.com/ssl/ptqrshow"
	ptqrLoginURL        = "https://xui.ptlogin2.qq.com/ssl/ptqrlogin"
	ptqrAID             = "716027609"
	ptqrDAID            = "381"
	ptqrThirdAID        = "102061775"
	u1URL               = "http://connect.qq.com"
	mcode               = "132f0a77d34402abc8463d60100011d19b0e"
	loginURLTemplate    = "https://xui.ptlogin2.qq.com/cgi-bin/xlogin?" +
		"pt_enable_pwd=1&appid=716027609&pt_3rd_aid=102061775&daid=381" +
		"&pt_skey_valid=0&style=35&force_qr=1&autorefresh=1" +
		"&s_url=http%3A%2F%2Fconnect.qq.com&refer_cgi=m_authorize" +
		"&ucheck=1&fall_to_wv=1&status_os=12" +
		"&redirect_uri=auth%3A%2F%2Ftauth.qq.com%2F" +
		"&client_id=102061775&pf=openmobile_android&response_type=token" +
		"&scope=all&sdkp=a&sdkv=3.5.17.lite" +
		"&sign=a6479455d3e49b597350f13f776a6288" +
		"&status_machine=MjMxMTdSSzY2Qw%3D%3D&switch=1" +
		"&time=1763280194&show_download_ui=true" +
		"&h5sig=trobryxo8IPM0GaSQH12mowKG-CY65brFzkK7_-9EW4" +
		"&loginty=6"
	userAgent = "mval/2.4.0.10053 Channel/10068 Manufacturer/Xiaomi Mozilla/5.0 (Linux; Android 14; 23117RK66C Build/V417IR; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/101.0.4951.61 Mobile Safari/537.36"
)

type Credentials struct {
	UserID      string `json:"userId"`
	TID         string `json:"tid"`
	OpenID      string `json:"openid"`
	UIN         int64  `json:"uin"`
	AccessToken string `json:"access_token"`
	CT          string `json:"ct,omitempty"`
	CTT         string `json:"ctt,omitempty"`
	SK          string `json:"sk,omitempty"`
}

type ProbeError struct{ Stage, Message string }

func (e *ProbeError) Error() string { return e.Message }

func buildAuthCookie(c Credentials, realToken bool) string {
	access := "null"
	if realToken {
		access = c.AccessToken
	}
	return strings.Join([]string{
		"clientType=9", "uin=o" + strconv.FormatInt(c.UIN, 10), "appid=102061775", "acctype=qc",
		"openid=" + c.OpenID, "access_token=" + access, "userId=" + c.UserID, "accountType=5", "tid=" + c.TID,
	}, "; ")
}

type ProbeConfig struct {
	APIBase      string
	MVALLoginURL string
	LoginTimeout time.Duration
	AutoOpen     bool
	TempDir      string
}

type Probe struct {
	cfg    ProbeConfig
	client *http.Client
}

func newProbe(cfg ProbeConfig) (*Probe, error) {
	if cfg.APIBase == "" {
		cfg.APIBase = defaultAPIBase
	}
	if cfg.MVALLoginURL == "" {
		cfg.MVALLoginURL = defaultMVALLoginURL
	}
	if cfg.LoginTimeout <= 0 {
		cfg.LoginTimeout = 50 * time.Second
	}
	if cfg.TempDir == "" {
		cfg.TempDir = "tmp"
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &Probe{cfg: cfg, client: &http.Client{Jar: jar, Timeout: 20 * time.Second}}, nil
}

func randomFraction() string {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		return fmt.Sprintf("0.%d", time.Now().UnixNano())
	}
	n := binary.LittleEndian.Uint64(b[:]) % 1_000_000_000_000_000
	return fmt.Sprintf("0.%015d", n)
}

func setHeaders(req *http.Request, headers map[string]string) {
	for k, v := range headers {
		req.Header.Set(k, v)
	}
}

func readHTTP(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return b, nil
}

func (p *Probe) do(method, rawURL string, headers map[string]string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return nil, err
	}
	setHeaders(req, headers)
	return p.client.Do(req)
}

func (p *Probe) cookieValue(name string) string {
	for _, raw := range []string{"https://xui.ptlogin2.qq.com/ssl/", "https://xui.ptlogin2.qq.com/", "https://openmobile.qq.com/", "https://connect.qq.com/"} {
		u, _ := url.Parse(raw)
		for _, c := range p.client.Jar.Cookies(u) {
			if c.Name == name {
				return c.Value
			}
		}
	}
	return ""
}

func loginHeaders() map[string]string {
	return map[string]string{
		"User-Agent": "Mozilla/5.0 (Linux; Android 12; 23117RK66C Build/V417IR; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/101.0.4951.61 Mobile Safari/537.36",
		"Accept":     "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", "Accept-Language": "zh-CN,zh;q=0.9",
		"Referer": "https://openmobile.qq.com/", "X-Requested-With": "com.tencent.apps.valorant", "Cookie": "accountType=5; clientType=9",
	}
}

var jsverREs = []*regexp.Regexp{
	regexp.MustCompile(`/monorepo/([0-9A-Za-z]+)/ptlogin/js/login_10\.js`),
	regexp.MustCompile(`/monorepo/([0-9A-Za-z]+)/ptlogin/js/`),
	regexp.MustCompile(`https://qq-web\.cdn-go\.cn/monorepo/([0-9A-Za-z]+)`),
}

func extractJSVer(html string) string {
	for _, re := range jsverREs {
		if m := re.FindStringSubmatch(html); len(m) > 1 {
			return m[1]
		}
	}
	return "28d22679"
}

func buildPTOpenLoginData(loginURL, idt string, nowMS int64) string {
	u, _ := url.Parse(loginURL)
	q := u.Query()
	get := func(k, d string) string {
		if v := q.Get(k); v != "" {
			return v
		}
		return d
	}
	pairs := [][2]string{
		{"which", ""}, {"refer_cgi", get("refer_cgi", "m_authorize")}, {"response_type", get("response_type", "token")}, {"client_id", get("client_id", ptqrThirdAID)},
		{"state", ""}, {"display", ""}, {"openapi", "1011"}, {"switch", get("switch", "1")}, {"src", "1"}, {"sdkv", get("sdkv", "3.5.17.lite")}, {"sdkp", get("sdkp", "a")},
		{"tid", idt}, {"pf", get("pf", "openmobile_android")}, {"need_pay", "0"}, {"browser", "0"}, {"browser_error", ""}, {"serial", ""}, {"token_key", ""},
		{"redirect_uri", get("redirect_uri", "auth://tauth.qq.com/")}, {"sign", get("sign", "")}, {"time", get("time", "")}, {"status_version", ""}, {"status_os", get("status_os", "12")},
		{"status_machine", get("status_machine", "")}, {"page_type", "1"}, {"has_auth", "1"}, {"update_auth", "1"}, {"auth_time", strconv.FormatInt(nowMS, 10)},
		{"loginfrom", ""}, {"h5sig", get("h5sig", "")}, {"loginty", get("loginty", "6")},
	}
	parts := make([]string, 0, len(pairs))
	for _, kv := range pairs {
		parts = append(parts, url.QueryEscape(kv[0])+"="+url.QueryEscape(kv[1]))
	}
	return strings.Join(parts, "&")
}

var authBodyRE = regexp.MustCompile(`auth://tauth\.qq\.com/[^\s"'<>]+`)
var callbackJSONRE = regexp.MustCompile(`(?s)_Callback\s*\(\s*(\{.*?\})\s*\)\s*;?\s*$`)
var bodyURLREs = []*regexp.Regexp{
	regexp.MustCompile(`ptuiCB\('[^']*','[^']*','([^']+)'`),
	regexp.MustCompile(`(?i)location\.href\s*=\s*['"]([^'"]+)['"]`),
	regexp.MustCompile(`(auth://tauth\.qq\.com/[^\s"'<>]+)`),
	regexp.MustCompile(`(https?://[^\s"'<>]*check_sig[^\s"'<>]*)`),
}

func extractAuthURLFromBody(body string) string {
	text := normalizeEscapedURL(body)
	if m := callbackJSONRE.FindStringSubmatch(text); len(m) > 1 {
		var obj map[string]any
		if json.Unmarshal([]byte(m[1]), &obj) == nil {
			v := stringValue(obj["url"])
			if strings.HasPrefix(v, "auth://") {
				return v
			}
		}
	}
	return authBodyRE.FindString(text)
}
func extractURLFromBody(body string) string {
	text := normalizeEscapedURL(body)
	for _, re := range bodyURLREs {
		if m := re.FindStringSubmatch(text); len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

func (p *Probe) QQLogin() (Credentials, error) {
	headers := loginHeaders()
	resp, err := p.do(http.MethodGet, loginURLTemplate, headers, nil)
	if err != nil {
		return Credentials{}, &ProbeError{"qq_xlogin", "QQ 登录页请求失败: " + err.Error()}
	}
	loginBytes, err := readHTTP(resp)
	if err != nil {
		return Credentials{}, &ProbeError{"qq_xlogin", "QQ 登录页请求失败: " + err.Error()}
	}
	loginPage := string(loginBytes)
	loginSig := p.cookieValue("pt_login_sig")
	idt := p.cookieValue("idt")
	if idt == "" {
		idt = strconv.FormatInt(time.Now().Unix(), 10)
	}
	jsver := extractJSVer(loginPage)
	ptOpen := buildPTOpenLoginData(loginURLTemplate, idt, time.Now().UnixMilli())
	aegis := p.cookieValue("__aegis_uid")
	if aegis == "" {
		sip, cip := p.cookieValue("pt_serverip"), p.cookieValue("pt_clientip")
		if sip != "" && cip != "" {
			aegis = sip + "-" + cip + "-4458"
		}
	}

	q := url.Values{"s": {"8"}, "e": {"0"}, "appid": {ptqrAID}, "type": {"0"}, "t": {randomFraction()}, "u1": {u1URL}, "daid": {ptqrDAID}, "pt_3rd_aid": {ptqrThirdAID}}
	qrHeaders := loginHeaders()
	qrHeaders["Referer"] = loginURLTemplate
	qrHeaders["Accept"] = "image/*"
	qrResp, err := p.do(http.MethodGet, ptqrShowURL+"?"+q.Encode(), qrHeaders, nil)
	if err != nil {
		return Credentials{}, &ProbeError{"qq_qr_request", "二维码请求失败: " + err.Error()}
	}
	qrBytes, err := readHTTP(qrResp)
	if err != nil {
		return Credentials{}, &ProbeError{"qq_qr_request", "二维码请求失败: " + err.Error()}
	}
	qrsig := p.cookieValue("qrsig")
	if qrsig == "" || len(qrBytes) == 0 {
		return Credentials{}, &ProbeError{"qq_qr_request", "二维码响应缺少 qrsig 或图片内容"}
	}
	if err := os.MkdirAll(p.cfg.TempDir, 0755); err != nil {
		return Credentials{}, err
	}
	qrPath := filepath.Join(p.cfg.TempDir, "qq_login.png")
	if err := os.WriteFile(qrPath, qrBytes, 0644); err != nil {
		return Credentials{}, err
	}
	defer os.Remove(qrPath)
	if p.cfg.AutoOpen {
		_ = openPath(qrPath)
	}

	token := calcPTQRToken(qrsig)
	deadline := time.Now().Add(p.cfg.LoginTimeout)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		params := url.Values{"u1": {u1URL}, "from_ui": {"1"}, "type": {"1"}, "ptlang": {"2052"}, "ptqrtoken": {strconv.FormatInt(token, 10)}, "daid": {ptqrDAID}, "aid": {ptqrAID}, "pt_3rd_aid": {ptqrThirdAID}, "pt_openlogin_data": {ptOpen}, "device": {"2"}, "ptopt": {"1"}, "pt_uistyle": {"35"}, "jsver": {jsver}, "r": {randomFraction()}}
		if loginSig != "" {
			params.Set("login_sig", loginSig)
		}
		if aegis != "" {
			params.Set("aegis_uid", aegis)
		}
		pollHeaders := map[string]string{"User-Agent": userAgent + " tencent_game_emulator", "Referer": loginURLTemplate, "Accept": "*/*", "X-Requested-With": "com.tencent.apps.valorant"}
		pr, err := p.do(http.MethodGet, ptqrLoginURL+"?"+params.Encode(), pollHeaders, nil)
		if err != nil {
			return Credentials{}, &ProbeError{"qq_poll", "扫码状态查询失败: " + err.Error()}
		}
		pb, err := readHTTP(pr)
		if err != nil {
			return Credentials{}, &ProbeError{"qq_poll", "扫码状态查询失败: " + err.Error()}
		}
		cb, ok := parsePTUICallback(string(pb))
		if !ok {
			continue
		}
		if cb.Code == "65" {
			return Credentials{}, &ProbeError{"qq_qr_expired", "二维码已失效，请重新运行"}
		}
		if cb.Code != "0" {
			continue
		}
		loginData, err := p.resolveCheckSig(cb.RedirectURL, loginURLTemplate)
		if err != nil {
			return Credentials{}, err
		}
		if loginData["openid"] != "" && loginData["access_token"] != "" {
			return p.exchangeMVAL(loginData["openid"], loginData["access_token"])
		}
	}
	return Credentials{}, &ProbeError{"qq_login_timeout", "等待扫码确认超时，请重新运行"}
}

func (p *Probe) resolveCheckSig(successURL, referer string) (map[string]string, error) {
	current := strings.TrimSpace(successURL)
	visited := map[string]bool{}
	noRedirect := *p.client
	noRedirect.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	for hop := 0; hop < 5; hop++ {
		if current == "" || visited[current] {
			break
		}
		visited[current] = true
		if strings.HasPrefix(current, "auth://") {
			return extractLoginData(current), nil
		}
		direct := extractLoginData(current)
		if direct["openid"] != "" && direct["access_token"] != "" {
			return direct, nil
		}
		req, err := http.NewRequest(http.MethodGet, current, nil)
		if err != nil {
			return nil, &ProbeError{"qq_check_sig", "QQ 授权跳转失败: " + err.Error()}
		}
		setHeaders(req, map[string]string{"User-Agent": userAgent, "Accept": "*/*", "Referer": referer})
		resp, err := noRedirect.Do(req)
		if err != nil && !errors.Is(err, http.ErrUseLastResponse) {
			return nil, &ProbeError{"qq_check_sig", "QQ 授权跳转失败: " + err.Error()}
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, &ProbeError{"qq_check_sig", "QQ 授权跳转失败: " + readErr.Error()}
		}
		if loc := strings.TrimSpace(resp.Header.Get("Location")); loc != "" {
			if !strings.HasPrefix(loc, "http://") && !strings.HasPrefix(loc, "https://") && !strings.HasPrefix(loc, "auth://") {
				base := resp.Request.URL
				if u, e := base.Parse(loc); e == nil {
					loc = u.String()
				}
			}
			current = loc
			continue
		}
		text := string(body)
		if a := extractAuthURLFromBody(text); a != "" {
			return extractLoginData(a), nil
		}
		if u := extractURLFromBody(text); u != "" && u != current {
			current = u
			continue
		}
		bd := extractLoginData(text)
		if bd["openid"] != "" && bd["access_token"] != "" {
			return bd, nil
		}
		break
	}
	return map[string]string{"openid": "", "access_token": "", "appid": "", "pay_token": "", "key": ""}, nil
}

func (p *Probe) apiPost(path string, creds Credentials, body map[string]any, stage string, realToken bool) (any, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	headers := map[string]string{"Accept": "*/*", "Accept-Language": "zh-CN,zh;q=0.9", "Content-Type": "application/json", "User-Agent": userAgent, "Connection": "keep-alive", "Cookie": buildAuthCookie(creds, realToken)}
	resp, err := p.do(http.MethodPost, p.cfg.APIBase+path, headers, strings.NewReader(string(raw)))
	if err != nil {
		return nil, &ProbeError{stage, "掌瓦接口请求失败: " + err.Error()}
	}
	b, err := readHTTP(resp)
	if err != nil {
		return nil, &ProbeError{stage, "掌瓦接口请求失败: " + err.Error()}
	}
	var payload map[string]any
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, &ProbeError{stage, "掌瓦接口返回了无法识别的数据结构"}
	}
	result := int64(0)
	if v := payload["result"]; v != nil {
		f, _ := strconv.ParseFloat(stringValue(v), 64)
		result = int64(f)
	}
	if result != 0 {
		msg := stringValue(firstMapValue(payload, "errMsg", "msg"))
		if msg == "" {
			msg = fmt.Sprintf("错误码 %d", result)
		}
		return nil, &ProbeError{stage, msg}
	}
	return payload["data"], nil
}

func (p *Probe) FetchMainRole(c Credentials) (any, error) {
	return p.apiPost("/go/account/get_main_role_raw", c, map[string]any{"_t": time.Now().Unix()}, "role_info", false)
}
func (p *Probe) FetchValCard(c Credentials, r RoleSnapshot) (any, error) {
	if r.GameRoleID == "" || r.Scene == "" {
		return nil, &ProbeError{"val_card", "角色信息缺少 game_role_id 或 scene"}
	}
	return p.apiPost("/go/mine/card/val_card", c, map[string]any{"game_role_id": r.GameRoleID, "scene": r.Scene, "_t": time.Now().Unix()}, "val_card", true)
}
func (p *Probe) FetchRecentMatches(c Credentials, r RoleSnapshot, pageNo, pageSize int) (any, error) {
	if r.GameRoleID == "" || r.Scene == "" {
		return nil, &ProbeError{"matches", "角色信息缺少 game_role_id 或 scene"}
	}
	if pageNo < 1 {
		pageNo = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	return p.apiPost("/go/agame/career/record/list?source_game_zone=agame&game_zone=agame", c, map[string]any{"game_role_id": r.GameRoleID, "scene": r.Scene, "page_no": pageNo, "page_size": pageSize, "_t": time.Now().Unix()}, "matches", true)
}

func (p *Probe) exchangeMVAL(openid, access string) (Credentials, error) {
	payload := map[string]any{"clienttype": 9, "config_params": map[string]any{"client_dev_name": "23117RK66C", "lang_type": 0}, "login_info": map[string]any{"appid": 102061775, "openid": openid, "qq_info_type": 5, "sig": access, "uin": 0}, "mappid": 10200, "mcode": mcode, "source_game_zone": "agame", "game_zone": "agame"}
	raw, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, p.cfg.MVALLoginURL, strings.NewReader(string(raw)))
	setHeaders(req, map[string]string{"Cookie": "clientType=9; openid=null; access_token=null;", "User-Agent": userAgent, "Content-Type": "application/json"})
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Credentials{}, &ProbeError{"mval_exchange", "掌瓦登录态交换失败: " + err.Error()}
	}
	b, err := readHTTP(resp)
	if err != nil {
		return Credentials{}, &ProbeError{"mval_exchange", "掌瓦登录态交换失败: " + err.Error()}
	}
	var data map[string]any
	if json.Unmarshal(b, &data) != nil {
		return Credentials{}, &ProbeError{"mval_exchange", "掌瓦登录态交换返回无效 JSON"}
	}
	if stringValue(data["result"]) != "0" {
		msg := stringValue(firstMapValue(data, "msg", "errMsg"))
		if msg == "" {
			msg = "未知错误"
		}
		return Credentials{}, &ProbeError{"mval_exchange", "掌瓦拒绝登录态交换: " + msg}
	}
	info := mapValue(mapValue(data["data"])["login_info"])
	if info == nil {
		return Credentials{}, &ProbeError{"mval_exchange", "掌瓦登录成功响应缺少 login_info"}
	}
	uid, tid := stringValue(info["user_id"]), stringValue(info["wt"])
	if uid == "" || tid == "" {
		return Credentials{}, &ProbeError{"mval_exchange", "掌瓦登录成功响应缺少 user_id 或 wt"}
	}
	uin, _ := strconv.ParseInt(stringValue(info["uin"]), 10, 64)
	return Credentials{UserID: uid, TID: tid, OpenID: openid, UIN: uin, AccessToken: access, CT: stringValue(info["ct"]), CTT: stringValue(info["ctt"]), SK: stringValue(info["sk"])}, nil
}
