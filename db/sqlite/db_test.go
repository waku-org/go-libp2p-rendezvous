package db

import (
	"bytes"
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestPackCookie(t *testing.T) {
	nonce := make([]byte, 16)
	_, err := rand.Read(nonce)
	if err != nil {
		t.Fatal(err)
	}

	counter := rand.Int63()
	ns := "blah"

	cookie := packCookie(counter, ns, nonce)

	if !validCookie(cookie, ns, nonce) {
		t.Fatal("packed an invalid cookie")
	}

	xcounter, err := unpackCookie(cookie)
	if err != nil {
		t.Fatal(err)
	}

	if counter != xcounter {
		t.Fatal("unpacked cookie counter not equal to original")
	}
}

func TestOpenCloseMemDB(t *testing.T) {
	db, err := OpenDB(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	// let the flush goroutine run its cleanup act
	time.Sleep(1 * time.Second)

	err = db.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestOpenCloseFSDB(t *testing.T) {
	db, err := OpenDB(context.Background(), "/tmp/rendezvous-test.db")
	if err != nil {
		t.Fatal(err)
	}

	nonce1 := db.nonce

	// let the flush goroutine run its cleanup act
	time.Sleep(1 * time.Second)

	err = db.Close()
	if err != nil {
		t.Fatal(err)
	}

	db, err = OpenDB(context.Background(), "/tmp/rendezvous-test.db")
	if err != nil {
		t.Fatal(err)
	}

	nonce2 := db.nonce

	// let the flush goroutine run its cleanup act
	time.Sleep(1 * time.Second)

	err = db.Close()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(nonce1, nonce2) {
		t.Fatal("persistent db nonces are not equal")
	}
}

func TestDBRegistrationAndDiscovery(t *testing.T) {
	db, err := OpenDB(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	p1, err := peer.Decode("QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH")
	if err != nil {
		t.Fatal(err)
	}

	p2, err := peer.Decode("QmUkUQgxXeggyaD5Ckv8ZqfW8wHBX6cYyeiyqvVZYzq5Bi")
	if err != nil {
		t.Fatal(err)
	}

	signedPeerRecord1 := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	signedPeerRecord2 := []byte{8, 7, 6, 5, 4, 3, 2, 1}

	// register p1 and do discovery
	_, err = db.Register(p1, "foo1", signedPeerRecord1, 60)
	if err != nil {
		t.Fatal(err)
	}

	count, err := db.CountRegistrations(p1)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("registrations for p1 should be 1")
	}

	rrs, cookie, err := db.Discover("foo1", nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rrs) != 1 {
		t.Fatal("should have got 1 registration")
	}
	rr := rrs[0]
	if rr.Id != p1 {
		t.Fatal("expected p1 ID in registration")
	}
	if !bytes.Equal(rr.SignedPeerRecord, signedPeerRecord1) {
		t.Fatal("expected p1's signed record in registration")
	}

	// register p2 and do progressive discovery
	_, err = db.Register(p2, "foo1", signedPeerRecord2, 60)
	if err != nil {
		t.Fatal(err)
	}

	count, err = db.CountRegistrations(p2)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("registrations for p2 should be 1")
	}

	rrs, cookie, err = db.Discover("foo1", cookie, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rrs) != 1 {
		t.Fatal("should have got 1 registration")
	}
	rr = rrs[0]
	if rr.Id != p2 {
		t.Fatal("expected p2 ID in registration")
	}
	if !bytes.Equal(rr.SignedPeerRecord, signedPeerRecord2) {
		t.Fatal("expected p2's addrs in registration")
	}

	// reregister p1 and do progressive discovery
	_, err = db.Register(p1, "foo1", signedPeerRecord1, 60)
	if err != nil {
		t.Fatal(err)
	}

	count, err = db.CountRegistrations(p1)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("registrations for p1 should be 1")
	}

	rrs, cookie, err = db.Discover("foo1", cookie, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rrs) != 1 {
		t.Fatal("should have got 1 registration")
	}
	rr = rrs[0]
	if rr.Id != p1 {
		t.Fatal("expected p1 ID in registration")
	}
	if !bytes.Equal(rr.SignedPeerRecord, signedPeerRecord1) {
		t.Fatal("expected p1's signed record in registration")
	}

	// do a full discovery
	rrs, _, err = db.Discover("foo1", nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rrs) != 2 {
		t.Fatal("should have got 2 registration")
	}
	rr = rrs[0]
	if rr.Id != p2 {
		t.Fatal("expected p2 ID in registration")
	}
	if !bytes.Equal(rr.SignedPeerRecord, signedPeerRecord2) {
		t.Fatal("expected p2's addrs in registration")
	}

	rr = rrs[1]
	if rr.Id != p1 {
		t.Fatal("expected p1 ID in registration")
	}
	if !bytes.Equal(rr.SignedPeerRecord, signedPeerRecord1) {
		t.Fatal("expected p1's signed record in registration")
	}

	// unregister p2 and redo discovery
	err = db.Unregister(p2, "foo1")
	if err != nil {
		t.Fatal(err)
	}

	count, err = db.CountRegistrations(p2)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("registrations for p2 should be 0")
	}

	rrs, _, err = db.Discover("foo1", nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rrs) != 1 {
		t.Fatal("should have got 1 registration")
	}
	rr = rrs[0]
	if rr.Id != p1 {
		t.Fatal("expected p1 ID in registration")
	}
	if !bytes.Equal(rr.SignedPeerRecord, signedPeerRecord1) {
		t.Fatal("expected p1's signed record in registration")
	}

	db.Close()
}

func TestDBRegistrationAndDiscoveryMultipleNS(t *testing.T) {
	db, err := OpenDB(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	p1, err := peer.Decode("QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH")
	if err != nil {
		t.Fatal(err)
	}

	p2, err := peer.Decode("QmUkUQgxXeggyaD5Ckv8ZqfW8wHBX6cYyeiyqvVZYzq5Bi")
	if err != nil {
		t.Fatal(err)
	}

	signedPeerRecord1 := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	signedPeerRecord2 := []byte{8, 7, 6, 5, 4, 3, 2, 1}

	_, err = db.Register(p1, "foo1", signedPeerRecord1, 60)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Register(p1, "foo2", signedPeerRecord1, 60)
	if err != nil {
		t.Fatal(err)
	}

	count, err := db.CountRegistrations(p1)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatal("registrations for p1 should be 2")
	}

	rrs, cookie, err := db.Discover("", nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rrs) != 2 {
		t.Fatal("should have got 2 registrations")
	}
	rr := rrs[0]
	if rr.Id != p1 {
		t.Fatal("expected p1 ID in registration")
	}
	if rr.Ns != "foo1" {
		t.Fatal("expected namespace foo1 in registration")
	}
	if !bytes.Equal(rr.SignedPeerRecord, signedPeerRecord1) {
		t.Fatal("expected p1's signed record in registration")
	}

	rr = rrs[1]
	if rr.Id != p1 {
		t.Fatal("expected p1 ID in registration")
	}
	if rr.Ns != "foo2" {
		t.Fatal("expected namespace foo1 in registration")
	}
	if !bytes.Equal(rr.SignedPeerRecord, signedPeerRecord1) {
		t.Fatal("expected p1's signed record in registration")
	}

	_, err = db.Register(p2, "foo1", signedPeerRecord2, 60)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Register(p2, "foo2", signedPeerRecord2, 60)
	if err != nil {
		t.Fatal(err)
	}

	count, err = db.CountRegistrations(p2)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatal("registrations for p2 should be 2")
	}

	rrs, cookie, err = db.Discover("", cookie, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rrs) != 2 {
		t.Fatal("should have got 2 registrations")
	}
	rr = rrs[0]
	if rr.Id != p2 {
		t.Fatal("expected p2 ID in registration")
	}
	if rr.Ns != "foo1" {
		t.Fatal("expected namespace foo1 in registration")
	}
	if !bytes.Equal(rr.SignedPeerRecord, signedPeerRecord2) {
		t.Fatal("expected p2's addrs in registration")
	}

	rr = rrs[1]
	if rr.Id != p2 {
		t.Fatal("expected p2 ID in registration")
	}
	if rr.Ns != "foo2" {
		t.Fatal("expected namespace foo1 in registration")
	}
	if !bytes.Equal(rr.SignedPeerRecord, signedPeerRecord2) {
		t.Fatal("expected p2's addrs in registration")
	}

	err = db.Unregister(p2, "")
	if err != nil {
		t.Fatal(err)
	}

	count, err = db.CountRegistrations(p2)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("registrations for p2 should be 0")
	}

	rrs, _, err = db.Discover("", nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rrs) != 2 {
		t.Fatal("should have got 2 registrations")
	}
	rr = rrs[0]
	if rr.Id != p1 {
		t.Fatal("expected p1 ID in registration")
	}
	if rr.Ns != "foo1" {
		t.Fatal("expected namespace foo1 in registration")
	}
	if !bytes.Equal(rr.SignedPeerRecord, signedPeerRecord1) {
		t.Fatal("expected p1's addrs in registration")
	}

	rr = rrs[1]
	if rr.Id != p1 {
		t.Fatal("expected p1 ID in registration")
	}
	if rr.Ns != "foo2" {
		t.Fatal("expected namespace foo1 in registration")
	}
	if !bytes.Equal(rr.SignedPeerRecord, signedPeerRecord1) {
		t.Fatal("expected p1's addrs in registration")
	}

	db.Close()
}

func TestDBCleanup(t *testing.T) {
	db, err := OpenDB(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	p1, err := peer.Decode("QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH")
	if err != nil {
		t.Fatal(err)
	}

	signedPeerRecord1 := []byte{1, 2, 3, 4, 5, 6, 7, 8}

	_, err = db.Register(p1, "foo1", signedPeerRecord1, 1)
	if err != nil {
		t.Fatal(err)
	}

	count, err := db.CountRegistrations(p1)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("registrations for p1 should be 1")
	}

	time.Sleep(2 * time.Second)

	db.cleanupExpired()

	count, err = db.CountRegistrations(p1)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("registrations for p1 should be 0")
	}

	rrs, _, err := db.Discover("foo1", nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rrs) != 0 {
		t.Fatal("should have got 0 registrations")
	}

	db.Close()
}
