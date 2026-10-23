// Package presence is the live driver supply: who is online, where, with which
// capabilities, and how fresh their last ping is. It lives in Redis only.
//
//	presence:{driver}   hash  lat lng h3 heading speed state services classes vehicle trip at
//	cell:{h3}           zset  member driver, score = last ping (unix seconds)
//	cells:active        set   cells touched recently (GC + live map)
//
// "Online" has exactly one definition: a presence record whose last ping is
// within the staleness window. Nothing in Postgres can disagree with it.
package presence
