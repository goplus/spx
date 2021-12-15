//
//  QNCommon.h
//  QNAudioRecorder
//
//  Created by 冯文秀 on 2021/12/7.
//

#import <Foundation/Foundation.h>
#import <AVFoundation/AVFoundation.h>

NS_ASSUME_NONNULL_BEGIN

@interface QNCommon : NSObject

+ (void)scaleWithSat:(AudioBuffer *)audioBuffer scale:(double)scale max:(float)max min:(float) min;

+ (double)calculateAudioBuffer:(AudioBuffer *)buffer overload:(int)overload;
@end

NS_ASSUME_NONNULL_END
