package coroutine

// enqueueJob injects scheduler work without publishing a blocked thread.
func (p *Coroutines) enqueueJob(job *WaitJob) {
	p.schedulerMu.Lock()
	p.currentJobs.PushBack(job)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

func joinYieldedOrDoneAll(co *Coroutines, threads []Thread) {
	for _, thread := range threads {
		co.JoinYieldedOrDone(thread)
	}
}
