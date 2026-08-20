package downloader

import (
	"context"
	"strings"
	"testing"

	"github.com/vavallee/bindery/internal/models"
)

// TestCheckDownloadClientHealth_NonIntrospectableTypesAreUnknown is the #2029
// report: SABnzbd, Transmission and Deluge used to store HealthOK with
// "Download client path check not required", asserting a check had passed that
// never ran. That is the mechanism behind "the client is fine and nothing ever
// imports". They cannot be introspected, so the honest answer is "unknown".
func TestCheckDownloadClientHealth_NonIntrospectableTypesAreUnknown(t *testing.T) {
	for _, clientType := range []string{"sabnzbd", "transmission", "deluge"} {
		t.Run(clientType, func(t *testing.T) {
			client := &models.DownloadClient{
				Type: clientType, Host: "127.0.0.1", Port: 1, Enabled: true,
			}
			got := CheckDownloadClientHealth(context.Background(), client, "/downloads", "", "")
			if got.Status != HealthUnknown {
				t.Fatalf("status = %q, want %q (message: %s)", got.Status, HealthUnknown, got.Message)
			}
			if strings.Contains(got.Message, "not required") {
				t.Errorf("message still claims the check was not required: %q", got.Message)
			}
			if !strings.Contains(got.Message, clientType) {
				t.Errorf("message %q does not name the client type", got.Message)
			}
		})
	}
}

// TestCheckDownloadClientHealth_UnreachableIntrospectableTypeIsUnknown: NZBGet
// and rTorrent do expose a completed path, but a probe that cannot connect must
// not be reported as a path error. CheckCompletedPathVisibility already treats
// an introspection failure as "can't tell" to avoid false alarms, and that has
// to survive being promoted into health.
func TestCheckDownloadClientHealth_UnreachableIntrospectableTypeIsUnknown(t *testing.T) {
	for _, clientType := range []string{"nzbget", "rtorrent"} {
		t.Run(clientType, func(t *testing.T) {
			client := &models.DownloadClient{
				Type: clientType, Host: "127.0.0.1", Port: 1, Enabled: true, Category: "books",
			}
			got := CheckDownloadClientHealth(context.Background(), client, "/downloads", "", "")
			if got.Status != HealthUnknown {
				t.Errorf("status = %q, want %q (message: %s)", got.Status, HealthUnknown, got.Message)
			}
		})
	}
}

// TestCheckDownloadClientHealth_NilClientIsUnknown: nil used to answer OK too.
func TestCheckDownloadClientHealth_NilClientIsUnknown(t *testing.T) {
	got := CheckDownloadClientHealth(context.Background(), nil, "/downloads", "", "")
	if got.Status != HealthUnknown {
		t.Errorf("status = %q, want %q", got.Status, HealthUnknown)
	}
}

// TestCheckingHealth_MessageIsNotQbittorrentSpecific: the placeholder written
// before every probe is now shown for every client type.
func TestCheckingHealth_MessageIsNotQbittorrentSpecific(t *testing.T) {
	got := CheckingHealth()
	if got.Status != HealthChecking {
		t.Errorf("status = %q, want %q", got.Status, HealthChecking)
	}
	if strings.Contains(strings.ToLower(got.Message), "qbittorrent") {
		t.Errorf("message is still qBittorrent-specific: %q", got.Message)
	}
}

// TestClientExpectedHint_NamesBothDirectoriesWhenAudiobooksAreSeparate covers
// the #1984 leftover: the hint is now built for every introspectable type, not
// just qBittorrent, and names both configured directories when the client
// serves audiobooks from a separate category.
func TestClientExpectedHint_NamesBothDirectoriesWhenAudiobooksAreSeparate(t *testing.T) {
	both := &models.DownloadClient{Category: "books", CategoryAudiobook: "audiobooks"}
	hint := clientExpectedHint(both, "/dl/books", "/dl/audio")
	if !strings.Contains(hint, "/dl/books") || !strings.Contains(hint, "/dl/audio") {
		t.Errorf("hint %q does not name both directories", hint)
	}

	single := &models.DownloadClient{Category: "books"}
	hint = clientExpectedHint(single, "/dl/books", "/dl/audio")
	if !strings.Contains(hint, "/dl/books") {
		t.Errorf("hint %q does not name the ebook directory", hint)
	}
	if strings.Contains(hint, "/dl/audio") {
		t.Errorf("hint %q names the audiobook directory for a client that does not serve one", hint)
	}
}

// TestRefreshDownloadClientHealthAsync_CoversEveryEnabledType: the refresh used
// to skip everything that was not qBittorrent, so five of six types were never
// probed at all. Disabled clients are still skipped.
func TestRefreshDownloadClientHealthAsync_CoversEveryEnabledType(t *testing.T) {
	store := NewHealthStore()
	clients := []models.DownloadClient{
		{ID: 1, Type: "sabnzbd", Host: "127.0.0.1", Port: 1, Enabled: true},
		{ID: 2, Type: "transmission", Host: "127.0.0.1", Port: 1, Enabled: true},
		{ID: 3, Type: "deluge", Host: "127.0.0.1", Port: 1, Enabled: false},
	}

	// The Checking placeholder is written synchronously before each probe is
	// launched, so its presence is a deterministic record of which clients were
	// considered, without waiting on the goroutines.
	RefreshDownloadClientHealthAsync(context.Background(), nil, store, clients, "/downloads", "", "")

	for _, id := range []int64{1, 2} {
		if store.Get(id) == nil {
			t.Errorf("client %d was never probed", id)
		}
	}
	if store.Get(3) != nil {
		t.Error("a disabled client was probed")
	}
}
