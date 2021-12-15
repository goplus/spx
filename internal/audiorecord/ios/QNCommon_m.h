//
//  QNCommon.m
//  QNAudioRecorder
//
//  Created by 冯文秀 on 2021/12/7.
//

#import "QNCommon.h"
#define kQNMixVolume 87.2984313

/*
 * The minimum audio level permitted.
 */
#define MIN_AUDIO_LEVEL -127
/*
 * The maximum audio level permitted.
 */
#define MAX_AUDIO_LEVEL 0


@implementation QNCommon

+ (void)scaleWithSat:(AudioBuffer *)audioBuffer scale:(double)scale max:(float)max min:(float) min {
    @autoreleasepool {
        if (audioBuffer->mDataByteSize == 0) {
            return;
        }
        
        if (scale > max) {
            scale = max;
        }
        if (scale < min) {
            scale = min;
        }
        
        short bufferByte[audioBuffer->mDataByteSize/2];
        memcpy(bufferByte, audioBuffer->mData, audioBuffer->mDataByteSize);
        
        // 将 buffer 内容取出，乘以 scale
        for (int i = 0; i < audioBuffer->mDataByteSize/2; i++) {
            bufferByte[i] = bufferByte[i]*scale;
        }
        memcpy(audioBuffer->mData, bufferByte, audioBuffer->mDataByteSize);
    }
}

+ (double)calculateAudioBuffer:(AudioBuffer *)buffer overload:(int)overload {
    double rms = 0;
    int length = buffer->mDataByteSize;
    short bufferByte[buffer->mDataByteSize/2];
    memcpy(bufferByte, buffer->mData, length);
    
    for (int i = 0; i < buffer->mDataByteSize/2; i++) {
        double sample = bufferByte[i];
        sample /= overload;
        rms += sample * sample;
    }
    rms = (length == 0) ? 0 : sqrt(rms / length);

    double db;

    if (rms > 0) {
        db = 20 * log10(rms);
        if (db < MIN_AUDIO_LEVEL){
            db = MIN_AUDIO_LEVEL;
        } else if (db > MAX_AUDIO_LEVEL){
            db = MAX_AUDIO_LEVEL;
        }
    } else {
        db = MIN_AUDIO_LEVEL;
    }
    double result = round(db);
    return (result + 127) / 127;
}

@end
