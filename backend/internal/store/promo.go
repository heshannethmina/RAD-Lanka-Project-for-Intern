package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Why a redemption can be refused. Three reasons rather than one because they
// need three different answers from a person: retype it, ask for a new one, or
// stop trying because you already have it.
var (
	// ErrPromoExhausted means the code hit its redemption ceiling.
	ErrPromoExhausted = errors.New("store: promo code fully redeemed")
	// ErrPromoExpired means the code is past its own expiry.
	ErrPromoExpired = errors.New("store: promo code expired")
	// ErrPromoAlreadyRedeemed means this account has already used this code.
	ErrPromoAlreadyRedeemed = errors.New("store: promo code already redeemed")
	// ErrPromoMisconfigured means the code names a tier that cannot be
	// granted — a typo in a hand-written row. Nobody's fault but ours, so it
	// must cost the person redeeming it nothing.
	ErrPromoMisconfigured = errors.New("store: promo code grants an unknown plan")
)

// PromoCode is a coupon that lifts an account's limits without a subscription.
type PromoCode struct {
	Code string
	// Plan is the tier the code grants, as a plan.Name value.
	Plan string
	// MaxRedemptions is how many people may use it; 0 means no ceiling.
	MaxRedemptions int
	Redemptions    int
	// ExpiresAt bounds the coupon, not the grant. nil means it never stops
	// being redeemable.
	ExpiresAt *time.Time
	// GrantDays is how long the grant lasts once redeemed; 0 means it does
	// not lapse.
	GrantDays int
	Note      string
	CreatedAt time.Time
}

// PromoGrant is what a redemption gave out.
type PromoGrant struct {
	Code string
	Plan string
	// ExpiresAt is nil for a grant that does not lapse.
	ExpiresAt *time.Time
}

// NormalizePromoCode puts a typed code into the one form the table stores.
//
// Upper-cased and stripped of all whitespace, because a code is read off a
// slide or out of an email and arrives with a stray space or a shifted caps
// lock more often than not. Refusing those would be a support ticket per
// customer for nothing.
func NormalizePromoCode(code string) string {
	var b strings.Builder
	for _, r := range code {
		if unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(unicode.ToUpper(r))
	}
	return b.String()
}

func scanPromo(p *PromoCode) []any {
	// grant_days is nullable and 0 already means "does not lapse", so it is
	// COALESCEd rather than carried as a pointer — same reasoning as the
	// user's promo text columns.
	return []any{
		&p.Code, &p.Plan, &p.MaxRedemptions, &p.Redemptions,
		&p.ExpiresAt, &p.GrantDays, &p.Note, &p.CreatedAt,
	}
}

// promoSelect is the column list every query here uses, in the order scanPromo
// expects. grant_days is folded to 0 when NULL.
const promoSelect = `code, plan, max_redemptions, redemptions,
	expires_at, COALESCE(grant_days, 0), note, created_at`

// CreatePromoCode writes a coupon.
//
// There is no admin UI, so in practice codes are written straight into the
// table with psql — this exists so that the normalisation and the constraints
// have one implementation, and so the tests do not have to hand-write SQL.
func (s *Store) CreatePromoCode(ctx context.Context, c PromoCode) (*PromoCode, error) {
	code := NormalizePromoCode(c.Code)

	var grantDays *int
	if c.GrantDays > 0 {
		grantDays = &c.GrantDays
	}

	var out PromoCode
	err := s.pool.QueryRow(ctx, `
		INSERT INTO promo_codes (code, plan, max_redemptions, expires_at, grant_days, note)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+promoSelect,
		code, c.Plan, c.MaxRedemptions, c.ExpiresAt, grantDays, c.Note,
	).Scan(scanPromo(&out)...)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, fmt.Errorf("store: create promo code: %w", err)
	}
	return &out, nil
}

// PromoCodeByCode reads one coupon. For support and for tests; the redemption
// path does its own locked read instead.
func (s *Store) PromoCodeByCode(ctx context.Context, code string) (*PromoCode, error) {
	var p PromoCode
	err := s.pool.QueryRow(ctx, `
		SELECT `+promoSelect+` FROM promo_codes WHERE code = $1
	`, NormalizePromoCode(code)).Scan(scanPromo(&p)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: promo code: %w", err)
	}
	return &p, nil
}

// RedeemPromoCode applies a coupon to an account and returns what it granted.
//
// The whole thing is one transaction over a locked coupon row, because every
// interesting failure here is a race: two people redeeming the last seat of a
// code at once, or one person double-clicking Redeem. SELECT ... FOR UPDATE
// serialises them, so the ceiling is a real ceiling rather than a suggestion.
//
// The unique key on promo_redemptions is the second half of that: without it a
// code with grant_days set could be redeemed again each morning to push the
// expiry out forever, quietly turning a 30-day trial into a permanent one.
//
// grantable vets the tier the coupon names, and it is a parameter rather than
// an import so this package stays ignorant of what a plan is. It is checked
// *inside* the transaction, before anything is written: codes are typed into
// the table by hand, and a typo in the plan column must roll the whole thing
// back rather than consume the one redemption this account will ever get.
// Pass nil to skip the check.
func (s *Store) RedeemPromoCode(ctx context.Context, code string, userID int64, grantable func(plan string) bool) (*PromoGrant, error) {
	code = NormalizePromoCode(code)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: begin redeem: %w", err)
	}
	// A rollback after a successful commit is a no-op, so this needs no flag.
	defer func() { _ = tx.Rollback(ctx) }()

	var p PromoCode
	err = tx.QueryRow(ctx, `
		SELECT `+promoSelect+`
		FROM promo_codes
		WHERE code = $1
		FOR UPDATE
	`, code).Scan(scanPromo(&p)...)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: read promo code: %w", err)
	}

	if grantable != nil && !grantable(p.Plan) {
		return nil, fmt.Errorf("%w: %q grants %q", ErrPromoMisconfigured, p.Code, p.Plan)
	}

	now := time.Now()
	if p.ExpiresAt != nil && !p.ExpiresAt.After(now) {
		return nil, ErrPromoExpired
	}
	if p.MaxRedemptions > 0 && p.Redemptions >= p.MaxRedemptions {
		return nil, ErrPromoExhausted
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO promo_redemptions (code, user_id) VALUES ($1, $2)
	`, code, userID)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return nil, ErrPromoAlreadyRedeemed
	}
	if err != nil {
		return nil, fmt.Errorf("store: record redemption: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE promo_codes SET redemptions = redemptions + 1 WHERE code = $1
	`, code); err != nil {
		return nil, fmt.Errorf("store: count redemption: %w", err)
	}

	// The expiry is stamped from the database's clock rather than Go's, so a
	// server with a skewed clock cannot hand out a grant that the queries
	// around it disagree about.
	grant := PromoGrant{Code: code, Plan: p.Plan}
	err = tx.QueryRow(ctx, `
		UPDATE users
		SET promo_code = $2,
		    promo_plan = $3,
		    promo_expires_at = CASE WHEN $4::INT > 0
		                            THEN now() + make_interval(days => $4::INT)
		                            ELSE NULL END
		WHERE id = $1
		RETURNING promo_expires_at
	`, userID, code, p.Plan, p.GrantDays).Scan(&grant.ExpiresAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: apply promo grant: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("store: commit redeem: %w", err)
	}
	return &grant, nil
}

// PromoCodeWithUse is a coupon plus who has taken it, for the admin listing.
type PromoCodeWithUse struct {
	PromoCode
	// Redeemers are the email addresses that claimed it, newest first.
	Redeemers []string
}

// PromoCodes lists every coupon, newest first, each with its redeemers.
//
// One query rather than a listing plus a lookup per row: the admin page shows
// both together, and a handful of coupons should not cost a handful of round
// trips. array_agg with a FILTER keeps codes that nobody has claimed, which an
// inner join would silently drop — and a code with no takers is exactly the
// one an operator is looking for when they wonder whether a link went out.
func (s *Store) PromoCodes(ctx context.Context, limit int) ([]PromoCodeWithUse, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.code, p.plan, p.max_redemptions, p.redemptions,
		       p.expires_at, COALESCE(p.grant_days, 0), p.note, p.created_at,
		       COALESCE(
		           array_agg(u.email ORDER BY r.redeemed_at DESC)
		               FILTER (WHERE u.email IS NOT NULL),
		           '{}'
		       ) AS redeemers
		FROM promo_codes p
		LEFT JOIN promo_redemptions r ON r.code = p.code
		LEFT JOIN users u ON u.id = r.user_id
		GROUP BY p.code
		ORDER BY p.created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list promo codes: %w", err)
	}
	defer rows.Close()

	var out []PromoCodeWithUse
	for rows.Next() {
		var c PromoCodeWithUse
		dest := append(scanPromo(&c.PromoCode), &c.Redeemers)
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("store: scan promo code: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list promo codes: %w", err)
	}
	return out, nil
}

// DeletePromoCode stops a coupon being claimed again.
//
// Grants already handed out are deliberately left alone: deleting a leaked
// coupon should not silently rewrite what people are already using. Taking
// those back is a separate, deliberate act — see ClearPromoGrants.
func (s *Store) DeletePromoCode(ctx context.Context, code string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM promo_codes WHERE code = $1`, NormalizePromoCode(code))
	if err != nil {
		return fmt.Errorf("store: delete promo code: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ClearPromoGrants revokes the grants a code handed out, and reports how
// many. Separate from DeletePromoCode because they are different decisions:
// one stops new claims, the other takes back access somebody already has.
func (s *Store) ClearPromoGrants(ctx context.Context, code string) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE users SET promo_code = NULL, promo_plan = NULL, promo_expires_at = NULL
		WHERE promo_code = $1
	`, NormalizePromoCode(code))
	if err != nil {
		return 0, fmt.Errorf("store: clear promo grants: %w", err)
	}
	return tag.RowsAffected(), nil
}
