// Package engine is the pure fare arithmetic: no database, no HTTP, no clock.
// It ports the reference system's V1 bracket engine (pricing_algo.py) exactly so historical
// fares can be reproduced, using decimals so results are stable to the cent.
//
//   - tariff.go   the tariff shape the engine reads
//   - fare.go     Quote (route fare) and Bill (completed trip) with the ordering
//     reference-system used: brackets → surge → floor → add-ons → booking fee
package engine
