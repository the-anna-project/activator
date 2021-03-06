package activator

import (
	"strings"
	"sync"

	"github.com/the-anna-project/configuration"
	"github.com/the-anna-project/context"
	currentbehaviour "github.com/the-anna-project/context/current/behaviour"
	currentsource "github.com/the-anna-project/context/current/source"
	"github.com/the-anna-project/event"
	"github.com/the-anna-project/instrumentor"
	"github.com/the-anna-project/permutation"
	"github.com/the-anna-project/random"
	"github.com/the-anna-project/worker"
)

var (
	configurationKey     = "activator/configuration"
	withStoredConfigsKey = "activator/with-stored-configs"
	withQueuedSignalsKey = "activator/with-queued-signals"
)

// ServiceConfig represents the configuration used to create a new CLG service.
type ServiceConfig struct {
	// Dependencies.
	ConfigurationService   configuration.Service
	EventCollection        *event.Collection
	InstrumentorCollection *instrumentor.Collection
	PermutationService     permutation.Service
	RandomService          random.Service
	WorkerService          worker.Service
}

// DefaultServiceConfig provides a default configuration to create a new CLG
// service by best effort.
func DefaultServiceConfig() ServiceConfig {
	var err error

	var configurationService configuration.Service
	{
		configurationConfig := configuration.DefaultServiceConfig()
		configurationService, err = configuration.NewService(configurationConfig)
		if err != nil {
			panic(err)
		}
	}

	var eventCollection *event.Collection
	{
		eventConfig := event.DefaultCollectionConfig()
		eventCollection, err = event.NewCollection(eventConfig)
		if err != nil {
			panic(err)
		}
	}

	var instrumentorCollection *instrumentor.Collection
	{
		instrumentorConfig := instrumentor.DefaultCollectionConfig()
		instrumentorCollection, err = instrumentor.NewCollection(instrumentorConfig)
		if err != nil {
			panic(err)
		}
	}

	var permutationService permutation.Service
	{
		permutationConfig := permutation.DefaultServiceConfig()
		permutationService, err = permutation.NewService(permutationConfig)
		if err != nil {
			panic(err)
		}
	}

	var randomService random.Service
	{
		randomConfig := random.DefaultServiceConfig()
		randomService, err = random.NewService(randomConfig)
		if err != nil {
			panic(err)
		}
	}

	var workerService worker.Service
	{
		workerConfig := worker.DefaultServiceConfig()
		workerService, err = worker.NewService(workerConfig)
		if err != nil {
			panic(err)
		}
	}

	config := ServiceConfig{
		// Dependencies.
		ConfigurationService:   configurationService,
		EventCollection:        eventCollection,
		InstrumentorCollection: instrumentorCollection,
		PermutationService:     permutationService,
		RandomService:          randomService,
		WorkerService:          workerService,
	}

	return config
}

// NewService creates a new configured CLG service.
func NewService(config ServiceConfig) (Service, error) {
	// Dependencies.
	if config.ConfigurationService == nil {
		return nil, maskAnyf(invalidConfigError, "configuration service must not be empty")
	}
	if config.EventCollection == nil {
		return nil, maskAnyf(invalidConfigError, "event collection must not be empty")
	}
	if config.InstrumentorCollection == nil {
		return nil, maskAnyf(invalidConfigError, "instrumentor collection must not be empty")
	}
	if config.PermutationService == nil {
		return nil, maskAnyf(invalidConfigError, "permutation service must not be empty")
	}
	if config.RandomService == nil {
		return nil, maskAnyf(invalidConfigError, "random service must not be empty")
	}
	if config.WorkerService == nil {
		return nil, maskAnyf(invalidConfigError, "worker service must not be empty")
	}

	newService := &service{
		// Dependencies.
		configuration: config.ConfigurationService,
		event:         config.EventCollection,
		instrumentor:  config.InstrumentorCollection,
		permutation:   config.PermutationService,
		random:        config.RandomService,
		worker:        config.WorkerService,

		// Internals.
		bootOnce:     sync.Once{},
		closer:       make(chan struct{}, 1),
		shutdownOnce: sync.Once{},
	}

	return newService, nil
}

type service struct {
	// Dependencies.
	configuration configuration.Service
	event         *event.Collection
	instrumentor  *instrumentor.Collection
	permutation   permutation.Service
	random        random.Service
	worker        worker.Service

	// Internals.
	bootOnce     sync.Once
	closer       chan struct{}
	shutdownOnce sync.Once
}

func (s *service) Boot() {
	s.bootOnce.Do(func() {
		// Service specific boot logic goes here.
	})
}

func (s *service) Execute(ctx context.Context, signal event.Signal) (event.Signal, error) {
	var err error

	var currentBehaviour currentbehaviour.Value
	{
		var ok bool
		currentBehaviour, ok = currentbehaviour.FromContext(signal.Context())
		if !ok {
			return nil, maskAnyf(invalidContextError, "current behaviour must not be empty")
		}
	}

	// Find the signal queue for the current CLG.
	var queue []event.Signal
	{
		// At first we add the current signal to the namespaced queue for the current
		// CLG, which is managed by the activator.
		err = s.event.Activator.Create(ctx, signal, currentBehaviour.ID)
		if err != nil {
			return nil, maskAny(err)
		}

		// The next step is to make sure we do not exceed a certain amount of queue
		// size. In case the signal queue for the current CLG exeeds a certain size,
		// it is unlikely that the queue is going to be helpful when growing any
		// further. Thus we cut the queue at some point beyond the interface
		// capabilities of the current CLG. Note that it is possible to have multiple
		// signals sent from the same source CLG.
		queueBuffer := len(currentBehaviour.Input.Types) + 1
		err := s.event.Activator.Limit(ctx, queueBuffer, currentBehaviour.ID)
		if err != nil {
			return nil, maskAny(err)
		}

		// We prepared the signal queue of the current CLG and can fetch all its
		// signals to process its activation.
		events, err := s.event.Activator.SearchAll(ctx, currentBehaviour.ID)
		if err != nil {
			return nil, maskAny(err)
		}

		queue, err = eventsToSignals(events)
		if err != nil {
			return nil, maskAny(err)
		}
	}

	// We have to synchronise the assigning process of the calculated results
	// below. To do so we make use of the configuration service which also aims to
	// learn from the experiences it makes over time. To leverage the
	// configuration service, we have to label our setting. The labels used here
	// scope the configuration to the activator service and the behaviour ID of
	// the currently requested CLG.
	var configLabels []string
	{
		configLabels = []string{
			configurationKey,
			currentBehaviour.ID,
		}
	}

	// Execute the activation rule actions.
	{
		actions := []func(ctx context.Context) error{
			func(ctx context.Context) error {
				newSignal, newQueue, err := s.WithStoredConfigs(ctx, signal, queue)
				if err != nil {
					return maskAny(err)
				}

				err = s.configuration.Create(ctx, configLabels, withStoredConfigsKey, []interface{}{newSignal, newQueue})
				if err != nil {
					return maskAny(err)
				}

				return nil
			},
			func(ctx context.Context) error {
				newSignal, newQueue, err := s.WithQueuedSignals(ctx, signal, queue)
				if err != nil {
					return maskAny(err)
				}

				err = s.configuration.Create(ctx, configLabels, withQueuedSignalsKey, []interface{}{newSignal, newQueue})
				if err != nil {
					return maskAny(err)
				}

				return nil
			},
		}

		errors := make(chan error, len(actions))
		executeConfig := s.worker.ExecuteConfig()
		executeConfig.Actions = actions
		executeConfig.Errors = errors
		executeConfig.NumWorkers = len(actions)
		err := s.worker.Execute(ctx, executeConfig)
		if allNotFound(errors) {
			// In case there did not any action find any result, we have to give up
			// and return the error.
			return nil, maskAnyf(notFoundError, "activation configuration")
		} else if IsNotFound(err) {
			// In case there did not all actions result in not found errors, we want
			// to ignore the errors and use the results we found so far.
		} else if err != nil {
			return nil, maskAny(err)
		}
	}

	// Fetch the actual results from the exexuted actions above.
	var newSignal event.Signal
	var newQueue []event.Signal
	{
		_, results, err := s.configuration.Execute(ctx, configLabels)
		if err != nil {
			return nil, maskAny(err)
		}
		newSignal, err = resultsToSignalWithIndex(results, 0)
		if err != nil {
			return nil, maskAny(err)
		}
		newQueue, err = resultsToQueueWithIndex(results, 1)
		if err != nil {
			return nil, maskAny(err)
		}
	}

	// Update the modified queue.
	{
		events := signalsToEvents(newQueue)
		err = s.event.Activator.WriteAll(ctx, events, currentBehaviour.ID)
		if err != nil {
			return nil, maskAny(err)
		}
	}

	return newSignal, nil
}

func (s *service) Failure(ctx context.Context, signal event.Signal) error {
	return nil
}

func (s *service) Shutdown() {
	s.shutdownOnce.Do(func() {
		close(s.closer)
	})
}

func (s *service) Success(ctx context.Context, signal event.Signal) error {
	return nil
}

func (s *service) WithStoredConfigs(ctx context.Context, signal event.Signal, queue []event.Signal) (event.Signal, []event.Signal, error) {
	var err error

	// Fetch the combinations of successful behaviour IDs which are known to be
	// useful for the activation of the requested CLG. The signals sent by the
	// CLGs being fetched here are known to be useful because they have already
	// been helpful for the execution of the current CLG in former CLG tree
	// executions.
	//
	// NOTE that we abuse the event service here for our configuration purposes.
	// There should be some decent configuration service in the future.
	var desiredBehaviourIDs [][]string
	{
		currentBehaviour, ok := currentbehaviour.FromContext(signal.Context())
		if !ok {
			return nil, nil, maskAnyf(invalidContextError, "current behaviour must not be empty")
		}
		events, err := s.event.Activator.SearchAll(ctx, currentBehaviour.ID)
		if event.IsNotFound(err) {
			return nil, nil, maskAnyf(notFoundError, "activation configuration")
		} else if err != nil {
			return nil, nil, maskAny(err)
		}
		for _, e := range events {
			desiredBehaviourIDs = append(desiredBehaviourIDs, strings.Split(e.Payload(), ","))
		}
	}

	// At this point we know some configurations for the requested CLG. Now we
	// have to find the combinations of queued signals that match the
	// configurations of the found desired behaviour IDs.
	var possibleMatches [][]event.Signal
	{
		for _, idList := range desiredBehaviourIDs {
			matches, err := s.matchPermutations(queue, idList, len(idList), len(idList), valuesToSourceIDs)
			if IsNotFound(err) {
				continue
			} else if err != nil {
				return nil, nil, maskAny(err)
			}
			possibleMatches = append(possibleMatches, matches...)
		}

		if len(possibleMatches) == 0 {
			return nil, nil, maskAnyf(notFoundError, "activation combination")
		}
	}

	// Now we have a list of signals which can be used to activate the requested
	// CLG. When we have multiple choices, we have to chose one. The chosen signal
	// combination has to be merged into one new signal and the signals being
	// combined to activate the requested CLG have to be removed from the signal
	// queue.
	var newSignal event.Signal
	var newQueue []event.Signal
	{
		newSignal, newQueue, err = s.filterMatches(queue, possibleMatches)
		if err != nil {
			return nil, nil, maskAny(err)
		}
	}

	return newSignal, newQueue, nil
}

func (s *service) WithQueuedSignals(ctx context.Context, signal event.Signal, queue []event.Signal) (event.Signal, []event.Signal, error) {
	var err error

	// Get the input types of the requested CLG to find out which signals we need
	// to activate the requested CLG.
	var currentBehaviour currentbehaviour.Value
	{
		var ok bool
		currentBehaviour, ok = currentbehaviour.FromContext(signal.Context())
		if !ok {
			return nil, nil, maskAnyf(invalidContextError, "current behaviour must not be empty")
		}
	}

	// At this point we know the input interface of the requested CLG. Now we have
	// to find the combinations of queued signals that match this interface.
	var possibleMatches [][]event.Signal
	{
		possibleMatches, err = s.matchPermutations(queue, currentBehaviour.Input.Types, len(currentBehaviour.Input.Types), 1, valuesToArgumentTypes)
		if IsNotFound(err) {
			return nil, nil, maskAnyf(notFoundError, "activation combination")
		} else if err != nil {
			return nil, nil, maskAny(err)
		}
	}

	// Now we have a list of signals which can be used to activate the requested
	// CLG. When we have multiple choices, we have to chose one. The chosen signal
	// combination has to be merged into one new signal and the signals being
	// combined to activate the requested CLG have to be removed from the signal
	// queue.
	var newSignal event.Signal
	var newQueue []event.Signal
	{
		newSignal, newQueue, err = s.filterMatches(queue, possibleMatches)
		if err != nil {
			return nil, nil, maskAny(err)
		}
	}

	// At last step we have to persists the source ID combination which resulted
	// out of the permuted signals above. The new signal's contect holds these
	// information. We create a new event and apply the list of source IDs as
	// configuration to the event's payload. Note that the order of the behaviour
	// IDs must be preserved, because it reflects the input interface of the
	// requested CLG.
	//
	// NOTE that we abuse the event service here for our configuration purposes.
	// There should be some decent configuration service in the future.
	{
		currentSource, ok := currentsource.FromContext(newSignal.Context())
		if !ok {
			return nil, nil, maskAnyf(invalidContextError, "behaviour id must not be empty")
		}
		newConfig := event.DefaultConfig()
		newConfig.Payload = strings.Join(currentSource.IDs, ",")
		newEvent, err := event.New(newConfig)
		if err != nil {
			return nil, nil, maskAny(err)
		}
		err = s.event.Activator.Create(ctx, newEvent, currentBehaviour.ID)
		if err != nil {
			return nil, nil, maskAny(err)
		}
	}

	return newSignal, newQueue, nil
}

func (s *service) filterMatches(queue []event.Signal, matches [][]event.Signal) (event.Signal, []event.Signal, error) {
	// Here we receive signal combinations that satisfy the interface of the
	// requested CLG. Now we need to select one random combination to cover all
	// possible combinations across all possible CLG trees being created over
	// time. This prevents us from choosing always only the first matching
	// combination, which would lack discoveries of all potential combinations
	// being created.
	//
	// TODO emit metrics about the decisions and successes/failures we make here.
	// TODO we should rather use the configuration service for this.
	matchIndex, err := s.random.CreateMax(len(matches))
	if err != nil {
		return nil, nil, maskAny(err)
	}
	selected := matches[matchIndex]

	// When we found a new matching list, we have to remove the matching signals
	// from the current queue.
	newQueue := removeSignals(queue, selected)

	// We now merge the matching signals to have one new signal that we can return
	// after queuing the dicsovered combination for the requested CLG.
	newSignal, err := event.NewSignalFromSignals(selected)
	if err != nil {
		return nil, nil, maskAny(err)
	}

	return newSignal, newQueue, nil
}

func (s *service) matchPermutations(queue []event.Signal, desiredList []string, maxGrowth, minGrowth int, converter func([]interface{}) ([]string, error)) ([][]event.Signal, error) {
	var matches [][]event.Signal

	// Prepare the permutation list to find out which combination of signals
	// satisfies the requested CLG's interface.
	permutationList, err := permutation.NewList(permutation.DefaultListConfig())
	if err != nil {
		return nil, maskAny(err)
	}
	permutationList.SetMaxGrowth(maxGrowth)
	permutationList.SetMinGrowth(minGrowth)
	permutationList.SetRawValues(queueToValues(queue))

	for {
		// Check if the current combination of signals already satisfies the
		// interface of the requested CLG. This is done in the first place to also
		// handle the very first combination of the permutation list, which is the
		// zero state of the permutation. In case there does a combination of
		// signals match the interface of the requested CLG, we capture the found
		// combination and try to find more combinations in the upcoming loops.
		permutedValues := permutationList.PermutedValues()
		// The permutation values are actually of type event.Signal. We need a
		// converter function for different purposes. Different algorithms rely on
		// different information which have to be aggregated in different ways.
		// Therefore the injected converter.
		currentList, err := converter(permutedValues)
		if err != nil {
			return nil, maskAny(err)
		}
		if equalStrings(desiredList, currentList) {
			newQueue, err := valuesToQueue(permutedValues)
			if err != nil {
				return nil, maskAny(err)
			}
			matches = append(matches, newQueue)
		}

		// Here we permute the list of the queued signals by one single permutation
		// step. As soon as the permutation list cannot be permuted anymore, we stop
		// the permutation loop to choose one random combination of the tracked list
		// in the next step below.
		err = s.permutation.PermuteBy(permutationList, 1)
		if permutation.IsMaxGrowthReached(err) {
			break
		} else if err != nil {
			return nil, maskAny(err)
		}
	}

	if len(matches) == 0 {
		return nil, maskAny(notFoundError)
	}

	return matches, nil
}
