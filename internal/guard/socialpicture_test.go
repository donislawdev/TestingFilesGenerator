package guard

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What this defends. The picture other sites show when this project is shared
// says what the card says today.
//
// Why it needed a guard, and it is not a hypothetical. The card is a PAGE -
// web/templates/social.html, rendered with the facts the registry holds, so the
// number of formats on it comes from the program. The picture is a PHOTOGRAPH
// of that page, taken by hand and committed as an asset. The site guard renders
// every page and compares it with what is published, and it COPIES the picture,
// so a card that changed and a picture that did not are both green.
//
// Measured 2026-09-02, and it had already happened: the committed picture said
// "21 formats" and "21 real formats" while the site said "24 real formats". The
// picture was taken on 2026-08-29 and the card was last rendered on 2026-08-31,
// when JPEG XL became the twenty fourth format. Three formats out of date, on
// the one image a stranger sees before they see anything else, for three days,
// with a green suite the whole time.
//
// How it works. The stamp beside the picture is the digest of the card the
// picture was taken of. Render the card now, hash it, compare. A template edit,
// a new format, a changed word - any of them moves the digest and this goes red
// until somebody takes the photograph again.
//
// What this does NOT check. That the picture is a photograph of THAT card
// rather than of something else - nothing here opens the PNG. A person pointing
// the camera at the wrong page would pass. It closes the drift, not the aim.
//
// Why a test cannot just take the photograph. It needs a browser, and the card
// has to be served over HTTP rather than opened as a file - it asks for its
// assets by absolute path, so under file:// the icon and the window shot are
// both missing and the result looks fine. That is why there is a probe with the
// trap written into it rather than four lines here.
const socialStampFile = "social-preview.sha256"

func TestTheSocialPictureShowsTheCardAsItIsNow(t *testing.T) {
	root := webRoot(t)

	picture := filepath.Join(root, "assets", "social-preview.png")
	if _, err := os.Stat(picture); err != nil {
		t.Fatalf("the social picture is missing: %v", err)
	}

	s := siteUnderTest(t)
	rendered, err := s.Render()
	if err != nil {
		t.Fatalf("rendering the site: %v", err)
	}
	card, ok := rendered["social.html"]
	if !ok {
		t.Fatalf("the site no longer renders social.html, so there is no card to "+
			"photograph. It renders: %d pages", len(rendered))
	}
	if len(card) == 0 {
		t.Fatal("the card rendered empty, so its digest would describe nothing")
	}

	sum := sha256.Sum256(card)
	now := hex.EncodeToString(sum[:])

	stamp := filepath.Join(root, socialStampFile)
	if os.Getenv("TFG_WRITE_SOCIAL_STAMP") == "1" {
		if err := os.WriteFile(stamp, []byte(now+"\n"), 0o644); err != nil {
			t.Fatalf("writing %s: %v", socialStampFile, err)
		}
		t.Logf("%s now says %s - only correct if the picture beside it was just retaken",
			socialStampFile, now)
		return
	}

	body, err := os.ReadFile(stamp)
	if err != nil {
		t.Fatalf("%s is missing, so nothing says which card the picture is of: %v",
			socialStampFile, err)
	}
	was := strings.TrimSpace(string(body))
	if was == "" {
		t.Fatalf("%s is empty, so this would pass whatever the card says", socialStampFile)
	}

	if was != now {
		t.Errorf("the social card has changed since the picture of it was taken.\n"+
			"  the picture is of: %s\n"+
			"  the card is now:   %s\n"+
			"The picture is what another site shows when somebody shares this project, and "+
			"nothing else notices it is stale - the site guard copies it rather than "+
			"rendering it. Measured once already: it sat three formats out of date for "+
			"three days.\n"+
			"Take it again, then rewrite the site and the stamp:\n"+
			"  python tools/probes/social-shot.py web/public web/assets/social-preview.png\n"+
			"  TFG_WRITE_SITE=1 go test ./internal/guard/ -run TestTheSiteSaysWhatTheToolSays\n"+
			"  TFG_WRITE_SOCIAL_STAMP=1 go test ./internal/guard/ -run TestTheSocialPicture",
			was, now)
	}
}
