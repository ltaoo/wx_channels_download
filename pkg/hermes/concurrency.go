package hermes

import (
	"context"
	"errors"
	"io"
	"sync"
)

// process_resources uses a fixed worker set and the engine-wide resource
// semaphore. The worker count is independent of the number of resources, so a
// large collection does not create one waiting goroutine per item.
func (d *HermesEngine) process_resources(
	ctx context.Context,
	resources []ResourceJob,
	worker_limit int,
	process func(index int, resource *ResourceJob),
) {
	if len(resources) == 0 || process == nil {
		return
	}
	worker_count := worker_limit
	global_limit := cap(d.resource_sem)
	if global_limit <= 0 {
		global_limit = 1
	}
	if worker_count <= 0 || worker_count > global_limit {
		worker_count = global_limit
	}
	if worker_count > len(resources) {
		worker_count = len(resources)
	}

	jobs := make(chan int)
	var workers sync.WaitGroup
	workers.Add(worker_count)
	for worker_index := 0; worker_index < worker_count; worker_index++ {
		go func() {
			defer workers.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case resource_index, ok := <-jobs:
					if !ok {
						return
					}
					if err := d.acquire_resource(ctx); err != nil {
						return
					}
					func() {
						defer d.release_resource()
						process(resource_index, &resources[resource_index])
					}()
				}
			}
		}()
	}

dispatch_loop:
	for resource_index := range resources {
		select {
		case jobs <- resource_index:
		case <-ctx.Done():
			break dispatch_loop
		}
	}
	close(jobs)
	workers.Wait()
}

func (d *HermesEngine) acquire_resource(ctx context.Context) error {
	select {
	case d.resource_sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

func (d *HermesEngine) release_resource() {
	<-d.resource_sem
}

func (d *HermesEngine) acquire_connection(ctx context.Context) error {
	select {
	case d.connection_sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

func (d *HermesEngine) release_connection() {
	<-d.connection_sem
}

func (d *HermesEngine) open_with_limit(
	ctx context.Context,
	driver ProtocolDriver,
	endpoint Endpoint,
	request ReadRequest,
) (io.ReadCloser, error) {
	if err := d.acquire_connection(ctx); err != nil {
		return nil, err
	}
	reader, err := driver.Open(ctx, endpoint, request)
	if err != nil {
		d.release_connection()
		return nil, err
	}
	if reader == nil {
		d.release_connection()
		return nil, errors.New("protocol driver returned a nil response body")
	}
	return &limited_read_closer{
		ReadCloser: reader,
		release:    d.release_connection,
	}, nil
}

type limited_read_closer struct {
	io.ReadCloser
	release_once sync.Once
	release      func()
}

func (r *limited_read_closer) Close() (err error) {
	defer r.release_once.Do(r.release)
	return r.ReadCloser.Close()
}
