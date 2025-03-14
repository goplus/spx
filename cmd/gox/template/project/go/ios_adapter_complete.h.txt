#ifndef IOS_ADAPTER_COMPLETE_H
#define IOS_ADAPTER_COMPLETE_H

// iOS environmental setup function
void initializeIOSEnvironment(void);

// Disable signal handling at iOS level
void disableIOSSignals(void);

// Configure thread priority and QoS
void configureIOSThreadPriority(void);

// Redirect stderr to iOS system logger
void redirectStderrToIOSSystemLogger(void);

#endif // IOS_ADAPTER_COMPLETE_H
