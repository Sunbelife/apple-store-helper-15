package services

import (
	"apple-store-helper/model"
	"context"
	"os"
	"testing"
	"time"
)

func TestLiveChengduLocation(t *testing.T) {
	if os.Getenv("APPLE_CHENGDU_LIVE") != "1" {
		t.Skip("opt-in public Apple pickup check")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	stock, err := QueryRetailPickup(ctx, modelAreaForTest("cn"), model.Store{StoreNumber: "R502", District: "610000"}, []model.Product{{Code: "MK2P4CH/A"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"R502", "R580"} {
		available, exists := stock[id+".MK2P4CH/A"]
		if !exists {
			t.Errorf("missing inventory for %s", id)
		}
		t.Logf("%s returned=%v available=%v", id, exists, available)
	}
}
