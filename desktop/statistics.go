package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

func (a *Desktop) GetStockStatistics(filters map[string]string) (map[string]interface{}, error) {
	query := url.Values{}
	for _, key := range []string{"date", "area", "model", "capacity", "color", "city", "recent"} {
		if v := filters[key]; v != "" {
			query.Set(key, v)
		}
	}
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Get("https://sunbelife.com/iphone-stock/api/stats?" + query.Encode())
	if err != nil {
		return nil, fmt.Errorf("统计暂时无法连接，请稍后刷新")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("统计服务返回 HTTP %d", resp.StatusCode)
	}
	var result map[string]interface{}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("统计数据格式无效")
	}
	return result, nil
}
