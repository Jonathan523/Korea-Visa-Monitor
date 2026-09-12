package visa

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Jonathan523/Korea-Visa-Monitor/internal/model"
	"golang.org/x/net/html"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"

type Client struct {
	URL  string
	HTTP *http.Client
}

func NewClient(endpoint string) *Client {
	return &Client{URL: endpoint, HTTP: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) Query(ctx context.Context, passportNumber, englishName, birthday string) (model.Result, error) {
	jarClient := *c.HTTP
	if jarClient.Jar == nil {
		jar, err := newCookieJar()
		if err != nil {
			return model.Result{}, err
		}
		jarClient.Jar = jar
	}

	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return model.Result{}, fmt.Errorf("创建签证页面请求失败: %w", err)
	}
	setBrowserHeaders(getReq)
	getResp, err := jarClient.Do(getReq)
	if err != nil {
		return model.Result{}, fmt.Errorf("访问签证页面失败: %w", err)
	}
	if err := closeResponse(getResp); err != nil {
		return model.Result{}, fmt.Errorf("访问签证页面失败: %w", err)
	}

	form := url.Values{
		"CMM_TEST_VAL": {"test"}, "sBUSI_GB": {"PASS_NO"},
		"sBUSI_GBNO": {passportNumber}, "ssBUSI_GBNO": {passportNumber},
		"pRADIOSEARCH": {"gb03"}, "sEK_NM": {strings.ToUpper(englishName)},
		"sFROMDATE": {birthday}, "sMainPopUpGB": {"main"},
		"TRAN_TYPE": {"ComSubmit"}, "SE_FLAG_YN": {""}, "LANG_TYPE": {"CH"},
	}
	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, strings.NewReader(form.Encode()))
	if err != nil {
		return model.Result{}, fmt.Errorf("创建签证查询请求失败: %w", err)
	}
	setBrowserHeaders(postReq)
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.Header.Set("Origin", "https://www.visa.go.kr")
	postReq.Header.Set("Referer", c.URL)
	postResp, err := jarClient.Do(postReq)
	if err != nil {
		return model.Result{}, fmt.Errorf("查询签证状态失败: %w", err)
	}
	defer postResp.Body.Close()
	if postResp.StatusCode < 200 || postResp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(postResp.Body, 4096))
		return model.Result{}, fmt.Errorf("查询签证状态失败: HTTP %s", postResp.Status)
	}
	result, err := Parse(postResp.Body)
	if err != nil {
		return model.Result{}, fmt.Errorf("解析签证查询结果失败: %w", err)
	}
	return result, nil
}

func Parse(r io.Reader) (model.Result, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return model.Result{}, err
	}
	area := findByID(doc, "result3_2")
	if area == nil {
		return model.Result{Message: "未找到 result3_2 查询结果区域"}, nil
	}
	applicationNumber := textByID(area, "ONLINE_APPL_NO")
	if applicationNumber == "" {
		return model.Result{Message: "未查询到签证申请记录"}, nil
	}
	return model.Result{
		Found: true, ApplicationNumber: applicationNumber,
		EntryPurpose: textByID(area, "ENTRY_PURPOSE"),
		Status:       textByID(area, "PROC_STS_CDNM_1"),
	}, nil
}

func findByID(n *html.Node, id string) *html.Node {
	if n.Type == html.ElementNode {
		for _, attr := range n.Attr {
			if attr.Key == "id" && attr.Val == id {
				return n
			}
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := findByID(child, id); found != nil {
			return found
		}
	}
	return nil
}

func textByID(parent *html.Node, id string) string {
	n := findByID(parent, id)
	if n == nil {
		return ""
	}
	var parts []string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			if value := strings.TrimSpace(node.Data); value != "" {
				parts = append(parts, value)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return strings.Join(parts, " ")
}

func setBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("DNT", "1")
}

func closeResponse(resp *http.Response) error {
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %s", resp.Status)
	}
	return nil
}
