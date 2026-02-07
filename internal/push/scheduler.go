package push

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dukerupert/gamwich/internal/model"
	"github.com/dukerupert/gamwich/internal/store"
)

// Scheduler periodically checks for notifications to send.
type Scheduler struct {
	mu       sync.RWMutex
	service  *Service
	push     *store.PushStore
	events   *store.EventStore
	chores   *store.ChoreStore
	members  *store.FamilyMemberStore
	interval time.Duration
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewScheduler creates a notification scheduler.
func NewScheduler(svc *Service, pushStore *store.PushStore, eventStore *store.EventStore, choreStore *store.ChoreStore, memberStore *store.FamilyMemberStore) *Scheduler {
	return &Scheduler{
		service:  svc,
		push:     pushStore,
		events:   eventStore,
		chores:   choreStore,
		members:  memberStore,
		interval: 60 * time.Second,
	}
}

// Start begins the scheduler loop.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	ctx, s.cancel = context.WithCancel(ctx)
	s.done = make(chan struct{})
	s.mu.Unlock()

	go func() {
		defer close(s.done)
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.tick()
			}
		}
	}()
}

// Stop gracefully stops the scheduler.
func (s *Scheduler) Stop() {
	s.mu.RLock()
	cancel := s.cancel
	done := s.done
	s.mu.RUnlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (s *Scheduler) tick() {
	householdIDs, err := s.push.ListHouseholdIDs()
	if err != nil {
		log.Printf("push scheduler: list households: %v", err)
		return
	}

	for _, hid := range householdIDs {
		s.checkCalendarReminders(hid)
		s.checkChoreDue(hid)
	}
}

func (s *Scheduler) checkCalendarReminders(householdID int64) {
	now := time.Now().UTC()
	windowEnd := now.Add(60 * time.Second)

	events, err := s.events.ListUpcomingWithReminders(now, windowEnd)
	if err != nil {
		log.Printf("push scheduler: calendar reminders: %v", err)
		return
	}

	for _, event := range events {
		if event.ReminderMinutes == nil {
			continue
		}
		leadTime := *event.ReminderMinutes
		refID := fmt.Sprintf("event-%d", event.ID)

		sent, err := s.push.WasSent(householdID, model.NotifTypeCalendarReminder, refID, leadTime)
		if err != nil {
			log.Printf("push scheduler: check sent: %v", err)
			continue
		}
		if sent {
			continue
		}

		// Determine who to notify: assigned member or all household members
		subs, err := s.push.ListByHousehold(householdID)
		if err != nil {
			log.Printf("push scheduler: list subs: %v", err)
			continue
		}

		payload := Payload{
			Title: "Calendar Reminder",
			Body:  fmt.Sprintf("%s starts in %d minutes", event.Title, leadTime),
			URL:   "/calendar",
			Tag:   fmt.Sprintf("calendar-%d", event.ID),
		}

		for _, sub := range subs {
			// If event is assigned to a specific member, only notify that user
			// For simplicity, notify all users who have the pref enabled
			enabled, _ := s.push.IsPreferenceEnabled(sub.UserID, householdID, model.NotifTypeCalendarReminder)
			if !enabled {
				continue
			}

			if err := s.service.Send(&sub, payload); err != nil {
				if errors.Is(err, ErrExpired) {
					s.push.DeleteByEndpoint(sub.Endpoint)
				} else {
					log.Printf("push scheduler: send calendar reminder: %v", err)
				}
			}
		}

		s.push.RecordSent(householdID, model.NotifTypeCalendarReminder, refID, leadTime)
	}
}

func (s *Scheduler) checkChoreDue(householdID int64) {
	now := time.Now().UTC()

	// Only run once per day at the start of each hour (minute 0)
	if now.Minute() != 0 {
		return
	}

	refID := fmt.Sprintf("chore-daily-%s", now.Format("2006-01-02"))
	sent, err := s.push.WasSent(householdID, model.NotifTypeChoreDue, refID, 0)
	if err != nil || sent {
		return
	}

	chores, err := s.chores.List()
	if err != nil {
		log.Printf("push scheduler: list chores: %v", err)
		return
	}

	if len(chores) == 0 {
		return
	}

	// Build a summary of chores with assignments
	var summaryItems []string
	for _, c := range chores {
		summaryItems = append(summaryItems, c.Title)
	}

	subs, err := s.push.ListByHousehold(householdID)
	if err != nil {
		log.Printf("push scheduler: list subs for chores: %v", err)
		return
	}

	body := fmt.Sprintf("You have %d chores to do today", len(summaryItems))
	if len(summaryItems) == 1 {
		body = fmt.Sprintf("Chore due today: %s", summaryItems[0])
	}

	payload := Payload{
		Title: "Chore Reminders",
		Body:  body,
		URL:   "/chores",
		Tag:   "chore-daily",
	}

	for _, sub := range subs {
		enabled, _ := s.push.IsPreferenceEnabled(sub.UserID, householdID, model.NotifTypeChoreDue)
		if !enabled {
			continue
		}

		if err := s.service.Send(&sub, payload); err != nil {
			if errors.Is(err, ErrExpired) {
				s.push.DeleteByEndpoint(sub.Endpoint)
			} else {
				log.Printf("push scheduler: send chore reminder: %v", err)
			}
		}
	}

	s.push.RecordSent(householdID, model.NotifTypeChoreDue, refID, 0)
}

// SendGroceryNotification sends a push notification for a grocery item addition.
// Called from the grocery handler, not from the scheduler.
func (s *Scheduler) SendGroceryNotification(householdID, excludeUserID int64, itemName string) {
	subs, err := s.push.ListByHousehold(householdID)
	if err != nil {
		log.Printf("push: grocery notification list subs: %v", err)
		return
	}

	payload := Payload{
		Title: "Grocery List Updated",
		Body:  fmt.Sprintf("%s was added to the grocery list", itemName),
		URL:   "/grocery",
		Tag:   "grocery-added",
	}

	for _, sub := range subs {
		if sub.UserID == excludeUserID {
			continue
		}
		enabled, _ := s.push.IsPreferenceEnabled(sub.UserID, householdID, model.NotifTypeGroceryAdded)
		if !enabled {
			continue
		}

		if err := s.service.Send(&sub, payload); err != nil {
			if errors.Is(err, ErrExpired) {
				s.push.DeleteByEndpoint(sub.Endpoint)
			} else {
				log.Printf("push: send grocery notification: %v", err)
			}
		}
	}
}
