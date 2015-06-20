package steam

import "time"

const (
	maxRequestsInFlight  = 100
	maxRequestsPerSecond = 100
)

type APIService struct {
	// Limiters for requests in flight and requests per second.
	inFlight  chan struct{}
	perSecond chan struct{}

	request         chan func()
	pendingRequests []func()

	quit chan struct{}
}

func (s *APIService) Start() {
	fillBucket(s.inFlight, maxRequestsInFlight)
	fillBucket(s.perSecond, maxRequestsPerSecond)
	t := time.NewTicker(time.Second)
	for {
		select {
		case <-t.C:
			fillBucket(s.perSecond, maxRequestsPerSecond)
			s.dispatchPendingRequests()

		case req := <-s.request:
			s.pendingRequests = append(s.pendingRequests, req)
			s.dispatchPendingRequests()
		case <-s.quit:
			return
		}
	}
}

func fillBucket(c chan<- struct{}, n int) {
	for i := 0; i < n; i++ {
		select {
		case c <- struct{}{}:
		default:
			return
		}
	}
}

func (s *APIService) dispatchPendingRequests() {
	for len(s.pendingRequests) > 0 {
		select {
		case <-s.inFlight:
		default:
			return
		}
		select {
		case <-s.perSecond:
		default:
			s.inFlight <- struct{}{}
			return
		}
		req := s.pendingRequests[0]
		go func() {
			req()
			s.inFlight <- struct{}{}
		}()
		s.pendingRequests = s.pendingRequests[1:]
	}
}

func (s *APIService) Stop() {
	close(s.quit)
}

func NewAPIService() *APIService {
	service := &APIService{
		inFlight:  make(chan struct{}, maxRequestsInFlight),
		perSecond: make(chan struct{}, maxRequestsPerSecond),
		request:   make(chan func()),
		quit:      make(chan struct{}),
	}
	go service.Start()
	return service
}
