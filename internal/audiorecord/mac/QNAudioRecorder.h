//
//  QNAudioRecorder.h
//  QNAudioRecorder
//
//  Created by tony.jing on 2021/12/8.
//

#import <Foundation/Foundation.h>

@class QNAudioRecorder;

@protocol QNAudioRecorderDelegate <NSObject>

/**
 * 录制音量回调
 *
 * @param audioReocrder QNAudioRecorder实例
 *
 * @param volume 录制音频音量 . 范围 [0 - 1.0]
 */
- (void)audioRecorder:(QNAudioRecorder *)audioReocrder onVolumeChanged:(double)volume;

@end

@interface QNAudioRecorder : NSObject

@property (nonatomic, strong) id<QNAudioRecorderDelegate> delegate;

/**
 * 开始录制
 *
 * @return QNAudioRecorder   注：成功时返回QNAudioRecorder实例;
 *                           失败时返回nil
 *
 * @warning 该方法使用到了系统内置麦克风设备，该设备为系统独占资源，请勿重复调用，如多次调用会失败并返回nil。
 */
+ (QNAudioRecorder *)start;

/**
 * 停止录制
 *
 * @return 是否成功停止录制   YES：成功    NO：失败
 */
- (BOOL)stop;
@end

