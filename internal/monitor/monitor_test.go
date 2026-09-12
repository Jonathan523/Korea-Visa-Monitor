package monitor

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Jonathan523/Korea-Visa-Monitor/internal/config"
	"github.com/Jonathan523/Korea-Visa-Monitor/internal/model"
)

type memoryStore struct {
	state     *model.State
	saveCount int
}

func (s *memoryStore) Load(context.Context) (*model.State, error) { return s.state, nil }
func (s *memoryStore) Save(_ context.Context, state model.State) error {
	s.state = &state
	s.saveCount++
	return nil
}

type fakeQuerier struct {
	result model.Result
	err    error
	calls  int
}

func (q *fakeQuerier) Query(context.Context, string, string, string) (model.Result, error) {
	q.calls++
	return q.result, q.err
}

type fakeNotifier struct {
	title, body string
	calls       int
}

func (n *fakeNotifier) Send(_ context.Context, title, body string) error {
	n.calls++
	n.title, n.body = title, body
	return nil
}

func testConfig() config.Config {
	loc := time.FixedZone("UTC+8", 8*60*60)
	start, _ := time.ParseInLocation("15:04", "08:00", loc)
	end, _ := time.ParseInLocation("15:04", "20:00", loc)
	return config.Config{PassportNumber: "P", EnglishName: "NAME", Birthday: "1990-01-01", WindowStart: start, WindowEnd: end, Location: loc}
}

func runAt(t *testing.T, state *model.State, result model.Result, hour int) (*memoryStore, *fakeQuerier, *fakeNotifier, string, error) {
	t.Helper()
	store := &memoryStore{state: state}
	query := &fakeQuerier{result: result}
	notifier := &fakeNotifier{}
	var output bytes.Buffer
	cfg := testConfig()
	runner := Runner{Config: cfg, Store: store, Querier: query, Notifier: notifier,
		Now: func() time.Time { return time.Date(2026, 9, 12, hour, 0, 0, 0, cfg.Location) }, Output: &output}
	err := runner.Run(context.Background())
	return store, query, notifier, output.String(), err
}

func TestRunnerFirstRunNotifiesAndSaves(t *testing.T) {
	result := model.Result{Found: true, ApplicationNumber: "A1", EntryPurpose: "Tour", Status: "审查中"}
	store, query, notifier, output, err := runAt(t, nil, result, 9)
	if err != nil {
		t.Fatal(err)
	}
	if query.calls != 1 || notifier.calls != 1 || store.saveCount != 1 || notifier.title != "签证监控已启动（首次运行）" {
		t.Fatalf("unexpected calls query=%d notify=%d save=%d title=%q", query.calls, notifier.calls, store.saveCount, notifier.title)
	}
	if !strings.Contains(output, "首次运行") {
		t.Fatalf("output = %q", output)
	}
}

func TestRunnerUnchangedDoesNotNotifyOrSave(t *testing.T) {
	result := model.Result{Found: true, ApplicationNumber: "A1", EntryPurpose: "Tour", Status: "审查中"}
	previous := &model.State{StatusText: result.StateString(), Summary: result.Summary()}
	store, _, notifier, _, err := runAt(t, previous, result, 9)
	if err != nil {
		t.Fatal(err)
	}
	if notifier.calls != 0 || store.saveCount != 0 {
		t.Fatalf("notify=%d save=%d", notifier.calls, store.saveCount)
	}
}

func TestRunnerIssuedStateSkipsQuery(t *testing.T) {
	previous := &model.State{Issued: true, StatusText: "found|A1|Tour|签发"}
	_, query, notifier, _, err := runAt(t, previous, model.Result{}, 9)
	if err != nil || query.calls != 0 || notifier.calls != 0 {
		t.Fatalf("err=%v query=%d notify=%d", err, query.calls, notifier.calls)
	}
}

func TestRunnerChangeToIssuedUsesCongratulations(t *testing.T) {
	previous := &model.State{StatusText: "found|A1|Tour|审查中", Summary: "旧状态"}
	result := model.Result{Found: true, ApplicationNumber: "A1", EntryPurpose: "Tour", Status: "签发 (2026.09.12.)"}
	store, _, notifier, _, err := runAt(t, previous, result, 9)
	if err != nil {
		t.Fatal(err)
	}
	if notifier.title != "恭喜！您的签证已被签发！" || !strings.Contains(notifier.body, "旧状态") || store.state == nil || !store.state.Issued {
		t.Fatalf("notify=%#v state=%#v", notifier, store.state)
	}
}

func TestRunnerOutsideWindowSkipsEverything(t *testing.T) {
	store, query, notifier, _, err := runAt(t, nil, model.Result{}, 7)
	if err != nil || query.calls != 0 || notifier.calls != 0 || store.saveCount != 0 {
		t.Fatalf("err=%v query=%d notify=%d save=%d", err, query.calls, notifier.calls, store.saveCount)
	}
}

func TestRunnerDoesNotPersistQueryFailure(t *testing.T) {
	cfg := testConfig()
	stateStore := &memoryStore{}
	query := &fakeQuerier{err: errors.New("site down")}
	runner := Runner{Config: cfg, Store: stateStore, Querier: query, Notifier: &fakeNotifier{},
		Now: func() time.Time { return time.Date(2026, 9, 12, 9, 0, 0, 0, cfg.Location) }, Output: &bytes.Buffer{}}
	if err := runner.Run(context.Background()); err == nil || stateStore.saveCount != 0 {
		t.Fatalf("err=%v save=%d", err, stateStore.saveCount)
	}
}
