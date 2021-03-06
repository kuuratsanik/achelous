package processor

import (
	"container/list"
	"context"
	"log"
	"os"
	"path"
	"strings"
	"sync"

	commonConfig "github.com/core-process/achelous/common/config"
	commonQueue "github.com/core-process/achelous/common/queue"
	config "github.com/core-process/achelous/upstream-core/config"

	"github.com/oklog/ulid"
)

func Run(cdata *config.Config, ctx context.Context) bool {

	log.Printf("queue processing started")

	// validate config
	if cdata.Target.Upload.URL == nil {
		log.Printf("no upload target url configured, aborting")
		return false
	}

	// only true if everything went fine
	OK := true

	// list of queues to be processes
	queues := list.New()
	queues.PushBack("")

	// initiate upload
	jobs := make(chan [2]string)
	var wg sync.WaitGroup

	go func() {
		// manage wait group
		wg.Add(1)
		defer wg.Done()

		// process jobs
		for job := range jobs {
			queueRef := commonQueue.QueueRef(job[0])
			msgID := ulid.MustParse(job[1])
			// run upload
			err := upload(cdata, queueRef, msgID)
			// handle errors
			if err != nil {
				log.Printf("upload of message %s in queue /%s failed: %v", msgID, queueRef, err)
				OK = false
				continue
			}
			// remove queue entry
			log.Printf("upload of message %s in queue /%s succeeded", msgID, queueRef)
			err = commonQueue.Remove(queueRef, msgID)
			if err != nil {
				log.Printf("could not remove message %s in queue /%s: %v", msgID, queueRef, err)
				OK = false
			}
		}
	}()

	// read file queues
	pext := "." + string(commonQueue.MessageStatusPreparing)
	qext := "." + string(commonQueue.MessageStatusQueued)

	for queues.Len() > 0 {
		// check if we have to exit early
		select {
		case <-ctx.Done():
			log.Printf("cancelling current queue walk")
			OK = false
			break
		default:
			// noop <=> non-blocking
		}

		// pop first element
		queue := queues.Remove(queues.Front()).(string)

		// open directory
		dir, err := os.Open(path.Join(commonConfig.Spool, queue))
		if err != nil {
			log.Printf("could not open queue /%s: %v", queue, err)
			OK = false
			continue
		}
		defer dir.Close()

		// read directory
		entries, err := dir.Readdirnames(-1)
		if err != nil {
			log.Printf("could not read entries from queue /%s: %v", queue, err)
			OK = false
			continue
		}

		// iterate entries
		for _, entry := range entries {
			// check if we have to exit early
			select {
			case <-ctx.Done():
				log.Printf("cancelling current entry iteration")
				OK = false
				break
			default:
				// noop <=> non-blocking
			}

			// get file info
			stat, err := os.Stat(path.Join(commonConfig.Spool, queue, entry))
			if err != nil {
				log.Printf("could not retrieve file info for entry %s in queue /%s: %v", entry, queue, err)
				// in case the error occured while stat'ing a potentially item in preparing
				// state, we will not include this as an invalid operation. this might happen
				// due to race conditions, which are happening by design in this case.
				if !strings.HasSuffix(entry, pext) {
					OK = false
				}
				continue
			}

			// handle entry
			if stat.Mode().IsDir() {
				// push to list of queues
				queues.PushBack(path.Join(queue, entry))

			} else if stat.Mode().IsRegular() {
				// push to upload channel (if queued item)
				if strings.HasSuffix(entry, qext) {
					id := entry[0 : len(entry)-len(qext)]
					jobs <- [2]string{queue, id}
				}
			}
		}
	}

	// wait for completion of uploads
	close(jobs)
	wg.Wait()

	// do not report success in case something did not work fine
	err := report(cdata, OK)
	if err != nil {
		log.Printf("could not report status: %v", err)
	}

	log.Printf("queue processing completed (OK=%v)", OK)
	return OK
}
