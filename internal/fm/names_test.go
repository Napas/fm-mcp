package fm

import (
	"slices"
	"sync"
	"testing"
)

func TestNormalizeFoldsCaseAndAccents(t *testing.T) {
	if got := normalize("Napas Mačiulaitis"); got != "napas maciulaitis" {
		t.Errorf("normalize = %q", got)
	}
	if got := normalize("Ismaïla SARR"); got != "ismaila sarr" {
		t.Errorf("normalize = %q", got)
	}
	for in, want := range map[string]string{
		"Martin Ødegaard": "martin odegaard", "Rasmus Højlund": "rasmus hojlund", "Łukasz Skorupski": "lukasz skorupski",
		"Đorđe": "dorde", "Ægir Þórsson": "aegir thorsson", "Weiß": "weiss", "Kılıç": "kilic", "Œuvre": "oeuvre", "Ħal Għargħur": "hal gharghur", "Col·legi": "col·legi",
	} {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNameWordsNormalizesAndJoinsHyphenatedParts(t *testing.T) {
	got := nameWords("Bradley", "Traoré-Barcola", "", "Spurs")
	want := []string{"bradley", "traore", "barcola", "traorebarcola", "spurs"}
	if !slices.Equal(got, want) {
		t.Errorf("nameWords = %q, want %q", got, want)
	}
}

func TestNormalizeIsSafeForConcurrentUse(t *testing.T) {
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 200 {
				if got := normalize("Mačiulaitis Ødegaard"); got != "maciulaitis odegaard" {
					t.Errorf("normalize = %q", got)
					return
				}
			}
		})
	}
	wg.Wait()
}
