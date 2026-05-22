package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/enetx/g"
	"github.com/enetx/surf"
)

const (
	ddKey     = "746B7C2640FFCBD6D2BEC599D9FB5F"
	ddVersion = "4.25.0"
	ddAPIURL  = "https://api-js.datadome.co/js/"
)

func fetchDatadomeCookie(client *surf.Client) (string, error) {
	jsData := buildJSData()
	jsBytes, err := json.Marshal(jsData)
	if err != nil {
		return "", err
	}

	body := fmt.Sprintf("ddv=%s&eventCounters=[]&jsType=ch&ddk=%s&request=%%2F&responsePage=origin&cid=null&Referer=%s&jsData=%s",
		ddVersion, ddKey, "https://www.sncf-connect.com", string(jsBytes))

	ua := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	headers := map[string]string{
		"content-type":             "application/x-www-form-urlencoded",
		"origin":                   "https://www.sncf-connect.com",
		"referer":                  "https://www.sncf-connect.com",
		"accept-encoding":          "gzip, deflate, br",
		"accept-language":          "fr-FR,fr;q=0.9,en;q=0.8",
		"sec-ch-ua":                `"Chromium";v="131", "Google Chrome";v="131", "Not_A Brand";v="24"`,
		"sec-ch-ua-mobile":         "?0",
		"sec-ch-ua-platform":       `"macOS"`,
		"sec-fetch-dest":           "empty",
		"sec-fetch-mode":           "cors",
		"sec-fetch-site":           "cross-site",
		"user-agent":               ua,
		"upgrade-insecure-requests": "1",
	}

	r := client.Post(g.String(ddAPIURL)).
		SetHeaders(headers).
		Body(body).
		Do()
	if r.IsErr() {
		return "", fmt.Errorf("dd cookie request: %w", r.Err())
	}
	resp := r.Unwrap()
	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return "", fmt.Errorf("dd cookie HTTP %d", int(resp.StatusCode))
	}

	var result struct {
		Status int    `json:"status"`
		Cookie string `json:"cookie"`
	}
	if err := resp.Body.JSON(&result); err != nil {
		return "", fmt.Errorf("dd cookie parse: %w", err)
	}
	if result.Cookie == "" {
		return "", fmt.Errorf("dd cookie empty (status %d)", result.Status)
	}

	cookie := strings.SplitN(result.Cookie, ";", 2)[0]
	return cookie, nil
}

func buildJSData() map[string]any {
	screens := [][2]int{{1920, 1080}, {2560, 1440}, {1440, 900}, {1680, 1050}, {1710, 1107}}
	idx := randInt(len(screens))
	sw, sh := screens[idx][0], screens[idx][1]
	oh := sh - randInt(80) - 40

	stcfp := fmt.Sprintf("ps://js.datadome.co/tags.js?id=%s:2:90854)\n    at https://js.datadome.co/tags.js?id=%s:2:53225", ddKey, ddKey)

	return map[string]any{
		"ttst":     fmt.Sprintf("%d.%d", 10+randInt(90), randBigInt()),
		"ifov":     "false",
		"hc":       pick([]int{4, 8, 12, 16}),
		"br_oh":    oh,
		"br_ow":    sw,
		"ua":       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		"wbd":      "false",
		"dp0":      "true",
		"tagpu":    fmt.Sprintf("%d.%d", 10+randInt(90), randBigInt()),
		"wdif":     "false",
		"wdifrm":   "false",
		"npmtm":    "false",
		"br_h":     sh,
		"br_w":     sw,
		"isf":      "false",
		"nddc":     1,
		"rs_h":     oh,
		"rs_w":     sw,
		"rs_cd":    24,
		"phe":      "false",
		"nm":       "false",
		"jsf":      "false",
		"lg":       "fr-FR",
		"pr":       2,
		"ars_h":    oh,
		"ars_w":    sw,
		"tz":       -60,
		"str_ss":   "true",
		"str_ls":   "true",
		"str_idb":  "true",
		"str_odb":  "false",
		"plgod":    "false",
		"plg":      5 + randInt(10),
		"plgne":    "true",
		"plgre":    "true",
		"plgof":    "false",
		"plggt":    "false",
		"pltod":    "false",
		"hcovdr":   "false",
		"hcovdr2":  "false",
		"plovdr":   "false",
		"plovdr2":  "false",
		"ftsovdr":  "false",
		"ftsovdr2": "false",
		"lb":       "false",
		"eva":      33,
		"lo":       "false",
		"ts_mtp":   0,
		"ts_tec":   "false",
		"ts_tsa":   "false",
		"vnd":      "Google Inc.",
		"bid":      "NA",
		"mmt":      "application/pdf,text/pdf",
		"plu":      "PDF Viewer,Chrome PDF Viewer,Chromium PDF Viewer,Microsoft Edge PDF Viewer,WebKit built-in PDF",
		"hdn":      "false",
		"awe":      "false",
		"geb":      "false",
		"dat":      "false",
		"med":      "defined",
		"aco":      "probably",
		"acots":    "false",
		"acmp":     "probably",
		"acmpts":   "true",
		"acw":      "probably",
		"acwts":    "false",
		"acma":     "maybe",
		"acmats":   "false",
		"acaa":     "probably",
		"acaats":   "true",
		"ac3":      "",
		"ac3ts":    "false",
		"acf":      "probably",
		"acfts":    "false",
		"acmp4":    "maybe",
		"acmp4ts":  "false",
		"acmp3":    "probably",
		"acmp3ts":  "false",
		"acwm":     "maybe",
		"acwmts":   "false",
		"ocpt":     "false",
		"vco":      "",
		"vcots":    "false",
		"vch":      "probably",
		"vchts":    "true",
		"vcw":      "probably",
		"vcwts":    "true",
		"vc3":      "maybe",
		"vc3ts":    "false",
		"vcmp":     "",
		"vcmpts":   "false",
		"vcq":      "",
		"vcqts":    "false",
		"vc1":      "probably",
		"vc1ts":    "true",
		"dvm":      pick([]int{4, 8, 16}),
		"sqt":      "false",
		"so":       "landscape-primary",
		"wdw":      "true",
		"cokys":    "bG9hZFRpbWVzY3NpYXBwL=",
		"ecpc":     "false",
		"lgs":      "true",
		"lgsod":    "false",
		"psn":      "true",
		"edp":      "true",
		"addt":     "true",
		"wsdc":     "true",
		"ccsr":     "true",
		"nuad":     "true",
		"bcda":     "true",
		"idn":      "true",
		"capi":     "false",
		"svde":     "false",
		"vpbq":     "true",
		"ucdv":     "false",
		"spwn":     "false",
		"emt":      "false",
		"bfr":      "false",
		"dbov":     "false",
		"cfpfe":    "ZnVuY3Rpb24oKXt2YXIgdD1kb2N1bWVudFsnXHg3MVx4NzVceDY1XHg3Mlx4NzlceDUzXHg2NVx4NmNceDY1XHg2M1x4NzRceDZmXHg3MiddKCdceDYyXHg3Mlx4NmZceDc3XHg3M1x4NjVceDcyXHg2Nlx4NmNceDZmXHg3N1x4MmRceDYzXHg2Zlx4NmVceDc0XHg2",
		"stcfp":    base64.StdEncoding.EncodeToString([]byte(stcfp)),
		"ckwa":     "true",
		"prm":      "true",
		"tzp":      "Europe/Paris",
		"cvs":      "true",
		"usb":      "defined",
		"glvd":     "Google Inc. (Apple)",
		"glrd":     "ANGLE (Apple, ANGLE Metal Renderer: Apple M1 Pro, Unspecified Version)",
		"wwl":      "false",
		"jset":     time.Now().Unix(),
	}
}

func randInt(max int) int {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return int(n.Int64())
}

func randBigInt() int64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(9000000000000))
	return n.Int64() + 1000000000000
}

func pick(choices []int) int {
	return choices[randInt(len(choices))]
}
