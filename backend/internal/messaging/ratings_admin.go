package messaging

import (
	"context"
	"time"
)

// ComplaintMaxScore: a rating at or below this is treated as a complaint on staff pages.
const ComplaintMaxScore = 2

// personRatingsLimit bounds each list on a person page.
const personRatingsLimit = 50

// RatingEntry is one rating with the trip and the other party, for staff.
type RatingEntry struct {
	TripID    string    `json:"trip_id"`
	TripCode  string    `json:"trip_code"`
	RaterRole string    `json:"rater_role"`
	Score     int       `json:"score"`
	Aspects   []string  `json:"aspects"`
	Comment   *string   `json:"comment,omitempty"`
	Other     *string   `json:"other_name,omitempty"` // who rated them, or whom they rated
	CreatedAt time.Time `json:"created_at"`
}

// PersonRatings is what a person was rated, how they rate others, and the
// low ratings both ways (complaints about them, complaints they made).
type PersonRatings struct {
	Received Summary `json:"received"`
	// Stars counts received ratings per score: Stars[0] is 1 star … Stars[4] is 5 stars.
	Stars           [MaxScore]int `json:"stars"`
	Given           Summary       `json:"given"`
	Recent          []RatingEntry `json:"recent"`           // received, newest first
	ComplaintsAbout []RatingEntry `json:"complaints_about"` // received at or below ComplaintMaxScore
	ComplaintsBy    []RatingEntry `json:"complaints_by"`    // given at or below ComplaintMaxScore
}

// PersonRatings reads a person's ratings both ways (staff rider and driver pages).
func (s *Service) PersonRatings(ctx context.Context, userID string) (*PersonRatings, error) {
	p := PersonRatings{}
	var err error
	if p.Received, p.Stars, err = s.store.received(ctx, userID); err != nil {
		return nil, err
	}
	if err = s.store.pool.QueryRow(ctx, `SELECT COALESCE(avg(score), 0)::float8, count(*) FROM trip_ratings WHERE rater_id=$1`, userID).
		Scan(&p.Given.Average, &p.Given.Count); err != nil {
		return nil, err
	}
	if p.Recent, err = s.store.ratingEntries(ctx, "r.ratee_id", userID, MaxScore); err != nil {
		return nil, err
	}
	if p.ComplaintsAbout, err = s.store.ratingEntries(ctx, "r.ratee_id", userID, ComplaintMaxScore); err != nil {
		return nil, err
	}
	p.ComplaintsBy, err = s.store.ratingEntries(ctx, "r.rater_id", userID, ComplaintMaxScore)
	return &p, err
}

func (s store) received(ctx context.Context, userID string) (Summary, [MaxScore]int, error) {
	var sm Summary
	var stars [MaxScore]int
	rows, err := s.pool.Query(ctx, `SELECT score, count(*) FROM trip_ratings WHERE ratee_id=$1 GROUP BY score`, userID)
	if err != nil {
		return sm, stars, err
	}
	defer rows.Close()
	total := 0
	for rows.Next() {
		var score, n int
		if err := rows.Scan(&score, &n); err != nil {
			return sm, stars, err
		}
		if score >= MinScore && score <= MaxScore {
			stars[score-1] = n
			sm.Count += n
			total += score * n
		}
	}
	if sm.Count > 0 {
		sm.Average = float64(total) / float64(sm.Count)
	}
	return sm, stars, rows.Err()
}

// ratingEntries lists ratings where col ("r.ratee_id" or "r.rater_id", never
// input) is the person, at or below maxScore, newest first.
func (s store) ratingEntries(ctx context.Context, col, userID string, maxScore int) ([]RatingEntry, error) {
	other := "r.rater_id"
	if col == "r.rater_id" {
		other = "r.ratee_id"
	}
	rows, err := s.pool.Query(ctx, `
		SELECT r.trip_id, t.code, r.rater_role, r.score, r.aspects, r.comment, u.full_name, r.created_at
		FROM trip_ratings r JOIN trips t ON t.id = r.trip_id LEFT JOIN users u ON u.id = `+other+`
		WHERE `+col+` = $1 AND r.score <= $2 AND t.brand_id = $3
		ORDER BY r.created_at DESC LIMIT $4`, userID, maxScore, s.brandID, personRatingsLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RatingEntry{}
	for rows.Next() {
		var e RatingEntry
		if err := rows.Scan(&e.TripID, &e.TripCode, &e.RaterRole, &e.Score, &e.Aspects, &e.Comment, &e.Other, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
