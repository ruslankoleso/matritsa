package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"matritsa/internal/seed"
	"matritsa/internal/store/db"
)

// Seed loads the synthetic companies, financials and demo accounts. It is idempotent.
func Seed(ctx context.Context, s *Store) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(seed.DemoPassword), 10)
	if err != nil {
		return err
	}
	return s.WithTx(ctx, func(q *db.Queries) error {
		for _, c := range seed.Companies() {
			co, err := q.UpsertCompany(ctx, db.UpsertCompanyParams{
				Inn: c.INN, Name: c.Name, Kind: c.Kind,
				RegisteredAt: pgtype.Date{Time: c.RegisteredAt, Valid: true},
				Okved:        c.OKVED, OkvedGroup: int32(c.OKVEDGroup), Director: c.Director, Address: c.Address,
				MonthlyObligations: c.MonthlyObligations,
			})
			if err != nil {
				return fmt.Errorf("company %s: %w", c.INN, err)
			}
			if err := SaveFinancials(ctx, q, co.ID, c.Revenue); err != nil {
				return err
			}
			coID := co.ID
			if _, err := q.UpsertUser(ctx, db.UpsertUserParams{
				Login: c.Login, PasswordHash: string(hash), Role: "client", CompanyID: &coID, FullName: c.OwnerName,
			}); err != nil {
				return fmt.Errorf("user %s: %w", c.Login, err)
			}
		}
		for _, st := range seed.StaffAccounts() {
			if _, err := q.UpsertUser(ctx, db.UpsertUserParams{
				Login: st.Login, PasswordHash: string(hash), Role: st.Role, FullName: st.FullName,
			}); err != nil {
				return fmt.Errorf("staff %s: %w", st.Login, err)
			}
		}
		return nil
	})
}

// SaveFinancials stores monthly revenue, oldest first, ending with the last full month.
func SaveFinancials(ctx context.Context, q *db.Queries, companyID string, revenue []int64) error {
	n := len(revenue)
	now := time.Now().UTC()
	for i, rev := range revenue {
		month := time.Date(now.Year(), now.Month()-time.Month(n-i), 1, 0, 0, 0, 0, time.UTC)
		if err := q.UpsertFinancial(ctx, db.UpsertFinancialParams{
			CompanyID: companyID, Month: pgtype.Date{Time: month, Valid: true}, Revenue: rev,
		}); err != nil {
			return err
		}
	}
	return nil
}
