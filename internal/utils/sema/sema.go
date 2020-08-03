package sema

// Sema represents a semaphore object
type Sema struct {
	rwLock chan bool
	ch     chan struct{}
}

// NewSema creates new semaphore object with given capacity
func NewSema(capacity uint) *Sema {
	ch := make(chan struct{}, capacity)
	rwLock := make(chan bool, 1)
	return &Sema{
		ch:     ch,
		rwLock: rwLock,
	}
}

// Cap returns the max amount of resources to acquire
func (sema *Sema) Cap() int {
	return cap(sema.ch)
}

// Len returns the amount of acquired resoures at the moment
func (sema *Sema) Len() int {
	return len(sema.ch)
}

// WaitToAcquire stops the current routine and resumes when the resource can be granted
func (sema *Sema) WaitToAcquire() {
	defer recover()
	sema.waitLock()
	sema.ch <- struct{}{}
}

// Release releases one unit of resource
func (sema *Sema) Release() {
	defer recover()
	sema.waitLock()
	_ = <-sema.ch
}

// ReleaseAll releases all taken resources
func (sema *Sema) ReleaseAll() {
	defer recover()
	sema.waitLock()
	// Drain the channel
	for {
		if _, ok := <-sema.ch; !ok {
			break
		}
	}
}

// AllocMore increases the capacity of semaphore on given amount
func (sema *Sema) AllocMore(addedCapacity uint) {
	curCap := sema.Cap()
	newCap := uint(curCap) + addedCapacity
	newChan := make(chan struct{}, newCap)

	sema.rwLock <- true
	curLen := sema.Len()
	for i := 0; i < curLen; i++ {
		newChan <- struct{}{}
	}
	sema.ch = newChan
	<-sema.rwLock
}

// Close blocks semaphore from granting new resources
func (sema *Sema) Close() {
	defer recover()
	close(sema.rwLock)
	close(sema.ch)
	sema.ch = nil
}

// IsLocked says if the semaphore is locked for grant or release at given moment
func (sema *Sema) IsLocked() bool {
	return len(sema.rwLock) > 0
}

func (sema *Sema) waitLock() {
	sema.rwLock <- true
	<-sema.rwLock
}
