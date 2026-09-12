package model

type Area struct {
	Title     string
	Locale    string
	ShortCode string
}

// Areas 地区列表 - 中国大陆、香港、日本、新加坡、美国、英国和澳大利亚
var Areas = []Area{
	{Title: "中国大陆", Locale: "zh_CN", ShortCode: "cn"},
	{Title: "香港", Locale: "zh_HK", ShortCode: "hk"},
	{Title: "日本", Locale: "ja_JP", ShortCode: "jp"},
	{Title: "新加坡", Locale: "en_SG", ShortCode: "sg"},
	{Title: "美国", Locale: "en_US", ShortCode: "us"},
	{Title: "英国", Locale: "en_GB", ShortCode: "uk"},
	{Title: "澳大利亚", Locale: "en_AU", ShortCode: "au"},
}
