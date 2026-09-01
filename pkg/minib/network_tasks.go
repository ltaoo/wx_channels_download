package minib

import "context"

func (runtime *page_runtime) begin_network_task() {
	if runtime != nil {
		runtime.pending_network_tasks.Add(1)
	}
}

func (runtime *page_runtime) complete_network_task(job func()) {
	if runtime == nil {
		return
	}
	completion := func() {
		defer runtime.pending_network_tasks.Add(-1)
		if job != nil {
			job()
		}
	}
	lifecycle_ctx := runtime.browser.lifecycle_ctx
	if lifecycle_ctx == nil {
		lifecycle_ctx = context.Background()
	}
	select {
	case runtime.external_jobs <- completion:
	case <-lifecycle_ctx.Done():
		runtime.pending_network_tasks.Add(-1)
	}
}

func (runtime *page_runtime) queue_external_job(job func()) {
	if runtime == nil || job == nil || runtime.external_jobs == nil {
		return
	}
	lifecycle_ctx := runtime.browser.lifecycle_ctx
	if lifecycle_ctx == nil {
		lifecycle_ctx = context.Background()
	}
	select {
	case runtime.external_jobs <- job:
	case <-lifecycle_ctx.Done():
	}
}

func (runtime *page_runtime) run_external_jobs(ctx context.Context) {
	if runtime == nil || runtime.external_jobs == nil {
		return
	}
	for callback_count := 0; callback_count < max_host_callbacks && ctx.Err() == nil; callback_count++ {
		if runtime.wait_condition_met() {
			return
		}
		select {
		case job := <-runtime.external_jobs:
			if job != nil {
				job()
			}
			if runtime.wait_condition_met() {
				return
			}
		default:
			return
		}
	}
}

func (runtime *page_runtime) wait_for_external_job(ctx context.Context) bool {
	if runtime == nil || runtime.external_jobs == nil {
		return false
	}
	select {
	case job := <-runtime.external_jobs:
		if job != nil {
			job()
		}
		return true
	case <-ctx.Done():
		return false
	}
}
