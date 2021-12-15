//
//  QNAudioRecorder.h
//  QNAudioRecorder
//
//  Created by 冯文秀 on 2021/12/7.
//

#import <Foundation/Foundation.h>

@class QNAudioRecorder;

@protocol QNAudioRecorderDelegate <NSObject>

/*!
 * @abstract 麦克风采集音频音量的回调
 *
 * @param audioRecorder QNAudioRecorder 实例
 *
 * @param volume 音量值，范围 0 ～ 1
 *
 * @since v1.0.0
 */
- (void)audioRecorder:(QNAudioRecorder *)audioRecorder volume:(double)volume;

@end

@interface QNAudioRecorder : NSObject

/*!
 * @abstract QNAudioRecorderDelegate 代理。
 *
 * @since v1.0.0
 */
@property (nonatomic, strong) id<QNAudioRecorderDelegate> delegate;

/*!
 * @abstract 开始录制。
 *
 * @warning 该方法使用到了系统内置麦克风设备，为系统独占资源，请勿重复调用，如多次调用会失败并返回 nil。
 *
 * @return QNAudioRecorder 实例，成功则返回 QNAudioRecorder 实例，失败则返回为空
 *
 * @since v1.0.0
 */
+ (QNAudioRecorder *)start;

/*!
 * @abstract 停止录制。
 *
 * @return 是否成功，返回 YES 则成功，NO 则失败
 *
 * @since v1.0.0
 */
- (BOOL)stop;

@end
