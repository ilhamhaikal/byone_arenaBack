package websocket

import (
	"testing"

	"github.com/google/uuid"
)

func TestCanReceiveFiltersByConsoleAndType(t *testing.T) {
	consoleA := uuid.New()
	consoleB := uuid.New()
	clientA := &client{consoleID: consoleA}
	clientB := &client{consoleID: consoleB}

	sessionA := NewEvent(EventSessionStarted, nil)
	sessionA.ConsoleID = &consoleA
	if !canReceive(clientA, sessionA) {
		t.Fatal("client konsol A harus menerima event konsol A")
	}
	if canReceive(clientB, sessionA) {
		t.Fatal("client konsol B tidak boleh menerima event konsol A")
	}

	payment := NewEvent(EventPaymentCreated, nil)
	if canReceive(clientA, payment) {
		t.Fatal("event pembayaran tidak boleh dikirim ke client TV")
	}

	notificationA := NewEvent(EventTVNotification, nil)
	notificationA.TargetConsoleIDs = []uuid.UUID{consoleA}
	if !canReceive(clientA, notificationA) {
		t.Fatal("notifikasi target A harus diterima client A")
	}
	if canReceive(clientB, notificationA) {
		t.Fatal("notifikasi target A tidak boleh diterima client B")
	}

	globalNotification := NewEvent(EventTVNotification, nil)
	if !canReceive(clientA, globalNotification) || !canReceive(clientB, globalNotification) {
		t.Fatal("notifikasi global harus diterima semua TV")
	}
}
