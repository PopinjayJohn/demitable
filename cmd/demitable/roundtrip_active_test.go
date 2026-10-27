package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/PopinjayJohn/demitable/internal/vault"
)

func TestRoundTripCreateSubmitApproveActive(t *testing.T) {
	for _, sys := range []string{"dnd5e", "blades_in_the_dark"} {
		t.Run(sys, func(t *testing.T) {
			dir, srv, gmToken := n6Setup(t)
			defer srv.Close()

			// mint a player account
			req, _ := http.NewRequest("POST", srv.URL+"/api/v1/players", bytes.NewBufferString(`{"name":"player1"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+gmToken)
			resp, _ := http.DefaultClient.Do(req)
			var pc map[string]any
			json.NewDecoder(resp.Body).Decode(&pc)
			resp.Body.Close()
			code := pc["join_code"].(string)
			r2, _ := http.Post(srv.URL+"/api/v1/auth/join", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"code":%q}`, code)))
			var jc map[string]any
			json.NewDecoder(r2.Body).Decode(&jc)
			r2.Body.Close()
			playerToken := jc["token"].(string)

			// create (draft)
			payload := map[string]any{"system": sys, "name": "RTChar-" + sys}
			b, _ := json.Marshal(payload)
			reqC, _ := http.NewRequest("POST", srv.URL+"/api/v1/characters", bytes.NewReader(b))
			reqC.Header.Set("Content-Type", "application/json")
			reqC.Header.Set("Authorization", "Bearer "+playerToken)
			respC, _ := http.DefaultClient.Do(reqC)
			var created map[string]any
			json.NewDecoder(respC.Body).Decode(&created)
			respC.Body.Close()
			if respC.StatusCode != http.StatusOK {
				t.Fatalf("create status %d", respC.StatusCode)
			}
			uid := created["uid"].(string)

			// submit
			reqS, _ := http.NewRequest("POST", srv.URL+"/api/v1/characters/"+uid+"/submit", nil)
			reqS.Header.Set("Authorization", "Bearer "+playerToken)
			respS, _ := http.DefaultClient.Do(reqS)
			io.Copy(io.Discard, respS.Body)
			respS.Body.Close()
			if respS.StatusCode != http.StatusOK {
				t.Fatalf("submit status %d", respS.StatusCode)
			}

			// approve
			reqA, _ := http.NewRequest("POST", srv.URL+"/api/v1/characters/"+uid+"/approve", nil)
			reqA.Header.Set("Authorization", "Bearer "+gmToken)
			respA, _ := http.DefaultClient.Do(reqA)
			var approved map[string]any
			json.NewDecoder(respA.Body).Decode(&approved)
			respA.Body.Close()
			if respA.StatusCode != http.StatusOK {
				t.Fatalf("approve status %d", respA.StatusCode)
			}
			if approved["workflow"] != "active" {
				t.Fatalf("approve response workflow=%v want active", approved["workflow"])
			}

			// final vault value
			v, err := vault.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			n := v.GetNote(uid)
			if n == nil || n.Workflow != "active" {
				got := interface{}(nil)
				if n != nil {
					got = n.Workflow
				}
				t.Fatalf("final workflow=%v want active", got)
			}
		})
	}
}
