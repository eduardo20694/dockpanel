package seats

import "testing"

func TestIndividualBlocksSecondSeat(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Ensure("default", 1); err != ErrSeatLimit {
		t.Fatalf("got %v", err)
	}
	if err := s.Save(Subscription{OrgID: "default", Plan: "organization", Seats: 3, Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Ensure("default", 1); err != nil {
		t.Fatal(err)
	}
	if err := s.Ensure("default", 3); err != ErrSeatLimit {
		t.Fatalf("got %v", err)
	}
	got := s.Get("default")
	if got.Plan != "organization" || got.Seats != 3 {
		t.Fatalf("%+v", got)
	}
}
