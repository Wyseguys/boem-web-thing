package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	bolt "go.etcd.io/bbolt"
)

func main() {
	dbPath := flag.String("db", "boem.db", "path to bolt db")
	flag.Parse()

	db, err := bolt.Open(*dbPath, 0o600, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(2)
	}
	defer db.Close()

	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("pages"))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var pi struct {
				URL    string `json:"url"`
				Status int    `json:"status"`
			}
			if err := json.Unmarshal(v, &pi); err != nil {
				return nil
			}
			if pi.Status >= 400 || pi.Status == 0 {
				fmt.Printf("%d\t%s\n", pi.Status, pi.URL)
			}
			return nil
		})
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error iterating pages: %v\n", err)
	}
}
