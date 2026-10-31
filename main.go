package main

import (
	"cms/koala"
	"os"
)

func main() {
	rows := []map[string]any{
		{
			"hs_code":              "100120012",
			"applicable_countries": []string{"AT", "CN"},
			"effective_at":         "2026-01-21",
			"excluding_countries": []map[string]any{
				{
					"country_code": "AU",
					"description":  "Australia",
				},
				{
					"country_code": "US",
					"description":  "United States of America",
				},
			},
			"lc_vats": []map[string]any{},
		},
		{
			"hs_code":              "450023234",
			"applicable_countries": []string{"IN", "LU", "MT"},
			"effective_at":         "2026-01-21",
			"excluding_countries": []map[string]any{
				{
					"country_code": "AU",
					"description":  "Australia",
				},
				{
					"country_code": "US",
					"description":  "United States of America",
				},
			},
			"lc_vats": []map[string]any{
				{
					"duty_name": "Values Added Tax",
					"duty":      "23 %",
					"lc_duty": []map[string]any{
						{"value": 23.00, "type": "AD_VALOREM"},
					},
					"lc_calculation_formula": "{1} >= 12",
				},
			},
		},
	}

	body, err := koala.Compress(rows)
	if err != nil {
		panic(err)
	}

	if err := os.WriteFile("body.xml.gz", body, 0644); err != nil {
		panic(err)
	}

}
