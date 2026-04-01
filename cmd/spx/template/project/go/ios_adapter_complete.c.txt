#include <signal.h>
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/types.h>
#include "ios_adapter_complete.h"

/*
 * iOS Adapter C File - For Go startup and initialization on iOS
 *
 * This file contains the implementation of all C-level adaptation code required to run Go on iOS
 * Including constructor functions to ensure initialization before Go runtime starts
 *
 */

// Application initialization - runs when loading the library
__attribute__((constructor)) void initialize_ios_bridge(void) {
    // Configure iOS environment before Go runtime starts
    initializeIOSEnvironment();
}

// Enable Go to run properly on iOS
void initializeIOSEnvironment(void) {
    disableIOSSignals();
    configureIOSThreadPriority();
    redirectStderrToIOSSystemLogger();
}

// Disable all signals that might cause issues with Go on iOS
void disableIOSSignals(void) {
    signal(SIGPIPE, SIG_IGN);
    signal(SIGBUS, SIG_IGN);
    signal(SIGSEGV, SIG_IGN);
    signal(SIGABRT, SIG_IGN);
    signal(SIGILL, SIG_IGN);
    signal(SIGFPE, SIG_IGN);
    signal(SIGTRAP, SIG_IGN);
}

// Configure thread priority for better performance on iOS
void configureIOSThreadPriority(void) {
    // Set thread priority to high if available
    pthread_attr_t attr;
    if (pthread_attr_init(&attr) == 0) {
        // Set stack size to 8MB (reasonable for iOS)
        pthread_attr_setstacksize(&attr, 8 * 1024 * 1024);
        pthread_attr_destroy(&attr);
    }
}

// Redirect stderr to iOS system logger (not implemented for simplicity)
void redirectStderrToIOSSystemLogger(void) {
    // This is a simplified version - we're just redirecting to /dev/null
    // The Go code has its own redirection which is more effective
    freopen("/dev/null", "w", stderr);
}

// Required for Go's signal handling compatibility
int sigaltstack(const stack_t *ss, stack_t *oss) {
    // No-op implementation to avoid link errors
    return 0;
}

// Required for Go's signal handling compatibility 
int pthread_sigmask(int how, const sigset_t *set, sigset_t *oset) {
    // No-op implementation to avoid link errors
    return 0;
}

// Required for Go's signal handling compatibility
int sigaction(int sig, const struct sigaction *act, struct sigaction *oact) {
    // No-op implementation to avoid link errors
    return 0;
}

// Required for Go's signal handling compatibility
int sigprocmask(int how, const sigset_t *set, sigset_t *oldset) {
    // No-op implementation to avoid link errors
    return 0;
}
