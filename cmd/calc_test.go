package main

import "testing"

func TestGetPoints(t *testing.T) {
	points, err := getPoints("../data/neth-point.json")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(points)
}
