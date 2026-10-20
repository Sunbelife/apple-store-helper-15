package model

type Area struct {
	Title             string
	Locale            string
	ShortCode         string
	PickupUnsupported bool
}

// Areas is the single registry for supported Apple storefronts and retail locales.
var Areas = []Area{
	{Title: "中国大陆", Locale: "zh_CN", ShortCode: "cn"},
	{Title: "香港", Locale: "zh_HK", ShortCode: "hk"},
	{Title: "日本", Locale: "ja_JP", ShortCode: "jp"},
	{Title: "新加坡", Locale: "en_SG", ShortCode: "sg"},
	{Title: "美国", Locale: "en_US", ShortCode: "us"},
	{Title: "英国", Locale: "en_GB", ShortCode: "uk"},
	{Title: "澳大利亚", Locale: "en_AU", ShortCode: "au"},
	{Title: "中国台湾", Locale: "zh_TW", ShortCode: "tw"},
	{Title: "韩国", Locale: "ko_KR", ShortCode: "kr"},
	{Title: "马来西亚", Locale: "en_MY", ShortCode: "my"},
	{Title: "泰国", Locale: "th_TH", ShortCode: "th"},
	{Title: "加拿大", Locale: "en_CA", ShortCode: "ca"},
	{Title: "德国", Locale: "de_DE", ShortCode: "de"},
	{Title: "法国", Locale: "fr_FR", ShortCode: "fr"},
	{Title: "意大利", Locale: "it_IT", ShortCode: "it"},
	{Title: "西班牙", Locale: "es_ES", ShortCode: "es"},
	{Title: "印度", Locale: "en_IN", ShortCode: "in"},
	{Title: "瑞士", Locale: "fr_CH", ShortCode: "ch"},
	{Title: "巴西", Locale: "pt_BR", ShortCode: "br", PickupUnsupported: true},
	{Title: "奥地利", Locale: "de_AT", ShortCode: "at"},
	{Title: "土耳其", Locale: "tr_TR", ShortCode: "tr"},
	{Title: "比利时", Locale: "fr_BE", ShortCode: "be"},
	{Title: "瑞典", Locale: "sv_SE", ShortCode: "se"},
	{Title: "阿联酋", Locale: "en_AE", ShortCode: "ae"},
	{Title: "墨西哥", Locale: "es_MX", ShortCode: "mx"},
	{Title: "荷兰", Locale: "nl_NL", ShortCode: "nl"},
}
