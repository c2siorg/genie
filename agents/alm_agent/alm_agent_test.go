package alm_agent

import "testing"

func TestBalancedNoBreach(t *testing.T) {
	bucket := BucketAmounts{Day1to7: 100, Day8to14: 100, Day15to30: 100, Day31to90: 100, Day91to180: 100, Day181to365: 100, Year1to3: 100, Year3to5: 100, Year5Plus: 100}
	res := New().Compute(Request{Assets: bucket, Liabilities: bucket, TotalAssetINR: 900})
	if res.HasBreach {
		t.Errorf("balanced book should not breach; got %+v", res)
	}
}

func TestSevereMaturityMismatchBreaches(t *testing.T) {
	// All assets long-dated, all liabilities short-dated.
	res := New().Compute(Request{
		Assets:        BucketAmounts{Year5Plus: 1_000},
		Liabilities:   BucketAmounts{Day1to7: 1_000},
		TotalAssetINR: 1_000,
	})
	if !res.HasBreach {
		t.Errorf("severe mismatch should breach; got %+v", res)
	}
}

func TestNIIPositiveWithAssetSensitiveBook(t *testing.T) {
	// More short-dated assets than liabilities → rising rates help NII.
	res := New().Compute(Request{
		Assets:      BucketAmounts{Day1to7: 200, Day31to90: 100},
		Liabilities: BucketAmounts{Day1to7: 50, Day31to90: 50},
		RateShockBp: 100, // +1%
	})
	if res.NIISensitivityINR <= 0 {
		t.Errorf("asset-sensitive book should benefit from rising rates; got %.2f", res.NIISensitivityINR)
	}
}

func TestNIINegativeWithLiabilitySensitiveBook(t *testing.T) {
	res := New().Compute(Request{
		Assets:      BucketAmounts{Day1to7: 50},
		Liabilities: BucketAmounts{Day1to7: 200},
		RateShockBp: 100,
	})
	if res.NIISensitivityINR >= 0 {
		t.Errorf("liability-sensitive book should lose NII on rising rates; got %.2f", res.NIISensitivityINR)
	}
}

func TestNineBucketsReturned(t *testing.T) {
	res := New().Compute(Request{})
	if len(res.Gaps) != 9 {
		t.Errorf("expected 9 buckets; got %d", len(res.Gaps))
	}
}
