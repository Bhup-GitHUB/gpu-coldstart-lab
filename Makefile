.PHONY: breakdown run-warm run-cold run-snapshot compare

breakdown:
	go run ./cmd/sim --breakdown

run-warm:
	go run ./cmd/sim --policy keep-warm

run-cold:
	go run ./cmd/sim --policy cold-start

run-snapshot:
	go run ./cmd/sim --policy snapshot

compare:
	go run ./cmd/sim --policy all
