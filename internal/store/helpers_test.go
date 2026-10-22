package store_test

import "matritsa/internal/store/db"

type dbQueries = db.Queries

func upsertUserParams() db.UpsertUserParams {
	return db.UpsertUserParams{Login: "x", PasswordHash: "h", Role: "client", FullName: "X"}
}
