package match

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"apaar-verify/internal/contracts"
	"github.com/jackc/pgx/v5"
)

// Matcher provides bounded fuzzy candidate matching (Rule #4 & #7)
type Matcher struct {
	config contracts.MatchConfig
}

// NewMatcher creates a Matcher instance with configured thresholds
func NewMatcher(cfg contracts.MatchConfig) *Matcher {
	if cfg.ConfigVersion == "" {
		cfg = contracts.DefaultMatchConfig()
	}
	return &Matcher{config: cfg}
}

// Normalize cleans names by trimming, lowercasing, and stripping non-alphanumeric runes
func Normalize(s string) string {
	var b strings.Builder
	lastSpace := false
	for _, r := range strings.TrimSpace(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			lastSpace = false
		} else if unicode.IsSpace(r) || r == '-' || r == '.' {
			if !lastSpace && b.Len() > 0 {
				b.WriteByte(' ')
				lastSpace = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// TrigramSimilarity calculates similarity score [0.0, 1.0] between two strings
func TrigramSimilarity(a, b string) float64 {
	normA := Normalize(a)
	normB := Normalize(b)
	if normA == normB {
		return 1.0
	}
	if len(normA) == 0 || len(normB) == 0 {
		return 0.0
	}

	triA := extractTrigrams(normA)
	triB := extractTrigrams(normB)

	if len(triA) == 0 || len(triB) == 0 {
		// Fallback to normalized prefix check for short strings (< 3 chars)
		if normA == normB {
			return 1.0
		}
		return 0.0
	}

	countA := make(map[string]int)
	for _, t := range triA {
		countA[t]++
	}

	intersection := 0
	for _, t := range triB {
		if countA[t] > 0 {
			intersection++
			countA[t]--
		}
	}

	total := len(triA) + len(triB) - intersection
	if total == 0 {
		return 0.0
	}
	return float64(intersection) / float64(total)
}

func extractTrigrams(s string) []string {
	// Pad string with spaces as PostgreSQL pg_trgm does
	padded := "  " + s + " "
	runes := []rune(padded)
	if len(runes) < 3 {
		return nil
	}
	trigrams := make([]string, 0, len(runes)-2)
	for i := 0; i <= len(runes)-3; i++ {
		trigrams = append(trigrams, string(runes[i:i+3]))
	}
	return trigrams
}

// FindCandidates executes bounded candidate matching against PostgreSQL (Non-negotiable Rule #4)
// Enforces:
// 1. Candidate blocking (s.dob = row.DOB and trigram index % match)
// 2. Minimum similarity score threshold
// 3. Strict maximum candidate limit (LIMIT :max_candidates)
// 4. Execution timeout (statement_timeout)
// 5. Zero all-pairs scan
func (m *Matcher) FindCandidates(
	ctx context.Context,
	tx pgx.Tx,
	row *contracts.CSVRow,
) ([]contracts.MatchCandidate, error) {
	if tx == nil || row == nil || row.APAARID == "" || row.StudentName == "" || row.DOB == "" {
		return nil, nil
	}

	timeout := time.Duration(m.config.QueryTimeoutMs) * time.Millisecond
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Parse DOB date
	dobDate, err := time.Parse("2006-01-02", strings.TrimSpace(row.DOB))
	if err != nil {
		return nil, nil // Malformed date already handled by V6
	}

	normName := Normalize(row.StudentName)

	// Bounded Candidate Blocking Query
	// Block: exact DOB match AND pg_trgm similarity on student name
	query := `
		SELECT s.apaar_id, s.name, s.dob, similarity(s.name, $1) AS score
		FROM students s
		WHERE s.apaar_id != $2
		  AND s.dob = $3
		  AND similarity(s.name, $1) >= $4
		ORDER BY score DESC
		LIMIT $5
	`

	rows, err := tx.Query(queryCtx, query,
		normName,
		row.APAARID,
		dobDate,
		m.config.MinScore,
		m.config.MaxCandidatesPerRow,
	)
	if err != nil {
		// If pg_trgm similarity function is unavailable in tests or query timed out, fail safely
		return nil, fmt.Errorf("candidate blocking query failed: %w", err)
	}
	defer rows.Close()

	var candidates []contracts.MatchCandidate
	for rows.Next() {
		var candAPAAR, candName string
		var candDOB time.Time
		var score float64

		if err := rows.Scan(&candAPAAR, &candName, &candDOB, &score); err == nil {
			cand := contracts.MatchCandidate{
				SourceRowOrEnrolmentRef: row.APAARID,
				CandidateEnrolmentRef:   candAPAAR,
				Score:                   score,
				MatchedFields:           []string{"dob", "student_name"},
				BlockingKey:             fmt.Sprintf("dob:%s", dobDate.Format("2006-01-02")),
				Method:                  "pg_trgm_similarity_with_dob_blocking",
				ThresholdsUsed: map[string]any{
					"min_score":      m.config.MinScore,
					"config_version": m.config.ConfigVersion,
					"max_candidates": m.config.MaxCandidatesPerRow,
				},
			}
			candidates = append(candidates, cand)
		}
	}

	return candidates, nil
}

