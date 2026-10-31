### Plan

create a package named koala which could take data of map[string]any datatype or generic struct datatype and returns body of the compressed xml file using gzip.

### Sample Data

`   
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
    		"lc_vats": []map[string]any{[]any},
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
    				"lc_calculation_formula": "{1}",
    			},
    		},
    	},
    }`

### Sample Output

`
<?xml version="1.0" encoding="UTF-8"?>

<rows>
    <row>
        <hs_code>100120012</hs_code>
        <applicable_countries>['AT', 'CN']</applicable_countries>
        <effective_at>2026-01-21</effective_at>
        <excluding_countries>[
            {
                'country_code': 'AU',
                'description': 'Australia',
            },
            {
                'country_code': 'US',
                'description': 'United States of America',
            },
        ]
</excluding_countries>
<lc_vats>[]</lc_vats>
    </row>
    <row>
        <hs_code>450023234</hs_code>
        <applicable_countries>['IN', 'LU', 'MT']</applicable_countries>
        <effective_at>2026-01-21</effective_at>
        <excluding_countries>[
            {
                'country_code': 'AU',
                'description': 'Australia',
            },
            {
                'country_code': 'US',
                'description': 'United States of America',
            },
        ]
</excluding_countries>
<lc_vats>[{'duty_name': 'Values Added Tax', 'duty': '23%', 'lc_duty': [{'value': 23.00, type: 'AD_VALOREM', 'lc_calculation_formula': '{1}'}]}]</lc_vats>
    </row>
</rows>
`
