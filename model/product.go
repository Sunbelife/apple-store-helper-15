package model

type Product struct {
	Title        string
	Type         string
	Code         string
	PurchasePath string
}

var TypeCode = map[string]string{
	"iphoneduo":      "A",
	"iphone18pro":    "A",
	"iphone18promax": "A",
	"iphone16":       "A",
	"iphone16plus":   "A",
	"iphone16pro":    "A",
	"iphone16promax": "A",
	"watchs12":       "A",
	"watchse3":       "A",
	"watchultra4":    "A",
}
