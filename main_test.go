package main

import "testing"

func TestReceiverBuilder(t *testing.T) {
	cases := []struct {
		Input    string
		Expected string
	}{
		{Input: "TestURL", Expected: "tu"},
		{Input: "testUrl", Expected: "tu"},
		{Input: "TestUrl", Expected: "tu"},
		{Input: "RandomEarthFact", Expected: "ref"},
		{Input: "RANDOMfact", Expected: "r"},
		{Input: "RANDOmFact", Expected: "rf"},
		{Input: "test", Expected: "t"},
	}
	for _, c := range cases {
		t.Run(c.Input, func(t *testing.T) {
			r := receiverBuilder(c.Input)
			if r != c.Expected {
				t.Logf("Expected %s, got %s", c.Expected, r)
				t.Fail()
			}
		})
	}
}
