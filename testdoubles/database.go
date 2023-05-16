package testdoubles

import (
	"context"

	"github.com/mbland/elistman/db"
	"github.com/mbland/elistman/types"
)

type Database struct {
	Subscribers         []*types.Subscriber
	SimulateGetErr      func(emailAddress string) error
	SimulatePutErr      func(emailAddress string) error
	SimulateDelErr      func(emailAddress string) error
	SimulateProcSubsErr func(emailAddress string) error
	Index               map[string]*types.Subscriber
}

func NewDatabase() *Database {
	simulateNilError := func(_ string) error {
		return nil
	}
	return &Database{
		Subscribers:         make([]*types.Subscriber, 0, 10),
		SimulateGetErr:      simulateNilError,
		SimulatePutErr:      simulateNilError,
		SimulateDelErr:      simulateNilError,
		SimulateProcSubsErr: simulateNilError,
		Index:               make(map[string]*types.Subscriber, 10),
	}
}

func (dbase *Database) Get(
	_ context.Context, email string,
) (sub *types.Subscriber, err error) {
	if err = dbase.SimulateGetErr(email); err != nil {
		return
	}

	var ok bool
	if sub, ok = dbase.Index[email]; !ok {
		err = db.ErrSubscriberNotFound
	}
	return
}

func (dbase *Database) Put(_ context.Context, sub *types.Subscriber) error {
	if err := dbase.SimulatePutErr(sub.Email); err != nil {
		return err
	}
	dbase.Subscribers = append(dbase.Subscribers, sub)
	dbase.Index[sub.Email] = sub
	return nil
}

func (dbase *Database) Delete(_ context.Context, email string) error {
	if err := dbase.SimulateDelErr(email); err != nil {
		return err
	}

	subIndex := -1

	for i, sub := range dbase.Subscribers {
		if sub.Email == email {
			subIndex = i
			break
		}
	}

	if subIndex == -1 {
		// Believe it or not, deleting a nonexistent record doesn't raise any
		// kind of an error. Confirmed by dynamodb_contract_test/
		// PutGetAndDeleteSucceed.
		return nil
	}

	before := dbase.Subscribers[:subIndex]
	after := dbase.Subscribers[subIndex+1:]
	dbase.Subscribers = append(before, after...)
	delete(dbase.Index, email)
	return nil
}

func (dbase *Database) ProcessSubscribersInState(
	_ context.Context, status types.SubscriberStatus, sp db.SubscriberProcessor,
) error {
	for _, sub := range dbase.Subscribers {
		if sub.Status != status {
			continue
		}

		err := dbase.SimulateProcSubsErr(sub.Email)
		if err != nil || !sp.Process(sub) {
			return err
		}
	}
	return nil
}
