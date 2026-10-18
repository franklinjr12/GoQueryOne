package app

import (
	"strings"
	"testing"

	"github.com/franklinjr12/GoQueryOne/internal/config"
)

func TestSettingsRoundTrip(t *testing.T) {
	q := config.QueryConfig{DefaultTimeoutMs: 60000, DefaultMaxRows: 1000, FetchPageSize: 500, StopOnErrorInScript: true}
	s := SettingsFromConfig(q)
	if s != (Settings{TimeoutSeconds: 60, MaxRows: 1000, PageSize: 500, StopOnError: true}) {
		t.Fatalf("SettingsFromConfig = %+v", s)
	}
	s.TimeoutSeconds, s.MaxRows, s.StopOnError = 5, 5000, false
	if err := s.Apply(&q); err != nil {
		t.Fatal(err)
	}
	if q.DefaultTimeoutMs != 5000 || q.DefaultMaxRows != 5000 || q.FetchPageSize != 500 || q.StopOnErrorInScript {
		t.Fatalf("Apply = %+v", q)
	}
}

func TestSettingsValidate(t *testing.T) {
	good := Settings{TimeoutSeconds: 30, MaxRows: 1000, PageSize: 100}
	cases := []struct {
		mod  func(*Settings)
		want string
	}{
		{func(s *Settings) { s.TimeoutSeconds = 0 }, "timeout"},
		{func(s *Settings) { s.TimeoutSeconds = MaxTimeoutSeconds + 1 }, "timeout"},
		{func(s *Settings) { s.MaxRows = 0 }, "maximum rows"},
		{func(s *Settings) { s.MaxRows = MaxRowsLimit + 1 }, "maximum rows"},
		{func(s *Settings) { s.PageSize = 0 }, "page size"},
	}
	for _, tc := range cases {
		s := good
		tc.mod(&s)
		q := config.QueryConfig{DefaultMaxRows: 7}
		err := s.Apply(&q)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%+v: err = %v, want %q", s, err, tc.want)
		}
		if q.DefaultMaxRows != 7 {
			t.Errorf("invalid settings must not change the config")
		}
	}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
}
