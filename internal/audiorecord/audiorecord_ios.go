//go:build ios
// +build ios

package audiorecord

/*
#cgo CFLAGS: -x objective-c  -I${SRCDIR}/ios -Wno-objc-missing-super-calls -Wno-arc-repeated-use-of-weak -Wimplicit-retain-self -Wduplicate-method-match -Wno-missing-braces -Wparentheses -Wswitch -Wunused-function -Wno-unused-label -Wno-unused-parameter
#cgo LDFLAGS: -framework Foundation  -framework AudioToolbox -framework UIKit -framework AVFAudio
#import <Foundation/Foundation.h>
#import "QNAudioRecorder_m.h"

@interface AudioRecorderSample : NSObject<QNAudioRecorderDelegate>
@property (nonatomic ,strong) QNAudioRecorder *audioRec;
@property double volume;
- (void)start;
- (void)stop;

@end

// Implementation
@implementation AudioRecorderSample

-(void)start{
	QNAudioRecorder *recorder = [QNAudioRecorder start];
    if (recorder) {
        self.audioRec = recorder;
        if (self.audioRec) {
            self.audioRec.delegate = self;
            NSLog(@"start record");
        }
    }else{
        NSLog(@"start record failed");
    }
}

-(void)stop{
	if(self.audioRec == NULL){
		return;
	}
	BOOL ret = [self.audioRec stop];
    if (ret) {
        NSLog(@"stop record");
    }else{
        NSLog(@"stop record failed");
    }
}
-(void)audioRecorder:(QNAudioRecorder *)audioReocrder volume:(double)volume{
	self.volume = volume;
}
@end

AudioRecorderSample *audio;
static void startRecorder(){
	if(audio != NULL){
		return;
	}
	audio= [[AudioRecorderSample alloc] init];
    [audio start];
}

static double getVolume(){
	if(audio == NULL){
		return 0.0;
	}
	return audio.volume;
}

static void stopRecorder(){
	if(audio == NULL){
		return;
	}
	[audio stop];
	audio=NULL;
}

*/
import "C"

func StartRecorder() {
	C.startRecorder()
}
func StopRecorder() {
	C.stopRecorder()
}
func GetVolume() float64 {
	vol := C.getVolume()
	return float64(vol)
}
