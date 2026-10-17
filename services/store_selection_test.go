package services

import (
	"apple-store-helper/model"
	"reflect"
	"testing"
)

func TestStoreSelectionByProvinceAndCity(t *testing.T) {
	area := Area.GetArea("中国大陆")
	s := storeService{stores: map[string][]model.Store{area.Locale: {
		{Province: "广东省", City: "深圳市", CityStoreName: "深圳益田假日广场"},
		{Province: "广东省", City: "广州市", CityStoreName: "广州天环"},
		{Province: "广东省", City: "深圳市", CityStoreName: "深圳罗湖"},
		{Province: "浙江省", City: "杭州市", CityStoreName: "杭州西湖"},
	}}}
	if got := s.ByLocationForOptions("中国大陆", "广东省", ""); len(got) != 3 {
		t.Fatalf("province selection: %v", got)
	}
	if got := s.ByLocationForOptions("中国大陆", "广东省", "深圳市"); len(got) != 2 {
		t.Fatalf("city selection: %v", got)
	}
	if got := s.ByLocationForOptions("中国大陆", "广东省", "杭州市"); len(got) != 0 {
		t.Fatalf("cross-province selection: %v", got)
	}
	if got := s.GetCitiesForProvince("中国大陆", "广东省"); !reflect.DeepEqual(got, []string{"广州市", "深圳市"}) {
		t.Fatalf("cities: %v", got)
	}
}
