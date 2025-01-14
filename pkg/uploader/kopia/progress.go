/*
Copyright The Velero Contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kopia

import (
	"sync/atomic"
	"time"

	"github.com/vmware-tanzu/velero/pkg/uploader"
)

//Throttle throttles controlle the interval of output result
type Throttle struct {
	throttle int64
	interval time.Duration
}

func (t *Throttle) ShouldOutput() bool {
	nextOutputTimeUnixNano := atomic.LoadInt64(&t.throttle)
	if nowNano := time.Now().UnixNano(); nowNano > nextOutputTimeUnixNano { //nolint:forbidigo
		if atomic.CompareAndSwapInt64(&t.throttle, nextOutputTimeUnixNano, nowNano+t.interval.Nanoseconds()) {
			return true
		}
	}
	return false
}

func (p *KopiaProgress) InitThrottle(interval time.Duration) {
	p.outputThrottle.throttle = 0
	p.outputThrottle.interval = interval
}

// KopiaProgress represents a backup or restore counters.
type KopiaProgress struct {
	// all int64 must precede all int32 due to alignment requirements on ARM
	// +checkatomic
	uploadedBytes int64 //the total bytes has uploaded
	cachedBytes   int64 //the total bytes has cached
	hashededBytes int64 //the total bytes has hashed
	// +checkatomic
	uploadedFiles int32 //the total files has ignored
	// +checkatomic
	ignoredErrorCount int32 //the total errors has ignored
	// +checkatomic
	fatalErrorCount     int32 //the total errors has occurred
	estimatedFileCount  int32 // +checklocksignore the total count of files to be processed
	estimatedTotalBytes int64 // +checklocksignore	the total size of files to be processed
	// +checkatomic
	processedBytes int64                           // which statistic all bytes has been processed currently
	outputThrottle Throttle                        // which control the frequency of update progress
	UpFunc         func(uploader.UploaderProgress) //which called by UpdateProgress func, it is used to update pvb or pvr status
}

//UploadedBytes the total bytes has uploaded currently
func (p *KopiaProgress) UploadedBytes(numBytes int64) {
	atomic.AddInt64(&p.uploadedBytes, numBytes)
	atomic.AddInt32(&p.uploadedFiles, 1)

	p.UpdateProgress()
}

//Error statistic the total Error has occurred
func (p *KopiaProgress) Error(path string, err error, isIgnored bool) {
	if isIgnored {
		atomic.AddInt32(&p.ignoredErrorCount, 1)
	} else {
		atomic.AddInt32(&p.fatalErrorCount, 1)
	}
}

//EstimatedDataSize statistic the total size of files to be processed and total files to be processed
func (p *KopiaProgress) EstimatedDataSize(fileCount int, totalBytes int64) {
	atomic.StoreInt64(&p.estimatedTotalBytes, totalBytes)
	atomic.StoreInt32(&p.estimatedFileCount, int32(fileCount))

	p.UpdateProgress()
}

//UpdateProgress which called by UpdateProgress func, it is used to update pvb or pvr status
func (p *KopiaProgress) UpdateProgress() {
	if p.outputThrottle.ShouldOutput() {
		p.UpFunc(uploader.UploaderProgress{
			TotalBytes: atomic.LoadInt64(&p.estimatedTotalBytes),
			BytesDone:  atomic.LoadInt64(&p.processedBytes),
		})
	}
}

//UploadStarted statistic the total Error has occurred
func (p *KopiaProgress) UploadStarted() {}

//CachedFile statistic the total bytes been cached currently
func (p *KopiaProgress) CachedFile(fname string, numBytes int64) {
	atomic.AddInt64(&p.cachedBytes, numBytes)
	p.UpdateProgress()
}

//HashedBytes statistic the total bytes been hashed currently
func (p *KopiaProgress) HashedBytes(numBytes int64) {
	atomic.AddInt64(&p.processedBytes, numBytes)
	atomic.AddInt64(&p.hashededBytes, numBytes)
	p.UpdateProgress()
}

//HashingFile statistic the file been hashed currently
func (p *KopiaProgress) HashingFile(fname string) {}

//ExcludedFile statistic the file been excluded currently
func (p *KopiaProgress) ExcludedFile(fname string, numBytes int64) {}

//ExcludedDir statistic the dir been excluded currently
func (p *KopiaProgress) ExcludedDir(dirname string) {}

//FinishedHashingFile which will called when specific file finished hash
func (p *KopiaProgress) FinishedHashingFile(fname string, numBytes int64) {
	p.UpdateProgress()
}

//StartedDirectory called when begin to upload one directory
func (p *KopiaProgress) StartedDirectory(dirname string) {}

//FinishedDirectory called when finish to upload one directory
func (p *KopiaProgress) FinishedDirectory(dirname string) {
	p.UpdateProgress()
}

//UploadFinished which report the files flushed after the Upload has completed.
func (p *KopiaProgress) UploadFinished() {
	p.UpdateProgress()
}

//ProgressBytes which statistic all bytes has been processed currently
func (p *KopiaProgress) ProgressBytes(processedBytes int64, totalBytes int64) {
	atomic.StoreInt64(&p.processedBytes, processedBytes)
	atomic.StoreInt64(&p.estimatedTotalBytes, totalBytes)
	p.UpdateProgress()
}
