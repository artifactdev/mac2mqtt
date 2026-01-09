//go:build darwin

package macos

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework IOKit

#import <Foundation/Foundation.h>
#import <IOKit/IOKitLib.h>

typedef double IOHIDFloat;
typedef struct __IOHIDEventSystemClient *IOHIDEventSystemClientRef;
typedef struct __IOHIDServiceClient *IOHIDServiceClientRef;
typedef struct __IOHIDEvent *IOHIDEventRef;

// Forward declarations
extern IOHIDEventSystemClientRef IOHIDEventSystemClientCreate(CFAllocatorRef allocator);
extern int IOHIDEventSystemClientSetMatching(IOHIDEventSystemClientRef client, CFDictionaryRef match);
extern CFArrayRef IOHIDEventSystemClientCopyServices(IOHIDEventSystemClientRef client);
extern IOHIDEventRef IOHIDServiceClientCopyEvent(IOHIDServiceClientRef service, int64_t type, int32_t options, int64_t depth);
extern IOHIDFloat IOHIDEventGetFloatValue(IOHIDEventRef event, int32_t field);
extern CFTypeRef IOHIDServiceClientCopyProperty(IOHIDServiceClientRef service, CFStringRef key);

#define kIOHIDEventTypeTemperature 15
#define IOHIDEventFieldBase(type) (type << 16)

// Helper function to create matching dictionary for temperature sensors
NSDictionary *createTemperatureMatching() {
    return @{
        @"PrimaryUsagePage" : @(0xff00),
        @"PrimaryUsage" : @(5)
    };
}

// Get temperature values from HID sensors with CPU and GPU separation
char *getHIDTemperatures() {
    @autoreleasepool {
        NSDictionary *matching = createTemperatureMatching();
        IOHIDEventSystemClientRef system = IOHIDEventSystemClientCreate(kCFAllocatorDefault);

        if (!system) {
            return strdup("");
        }

        IOHIDEventSystemClientSetMatching(system, (__bridge CFDictionaryRef)matching);
        CFArrayRef servicesRef = IOHIDEventSystemClientCopyServices(system);
        NSArray *services = (__bridge NSArray *)servicesRef;

        if (!services || [services count] == 0) {
            if (servicesRef) CFRelease(servicesRef);
            CFRelease(system);
            return strdup("");
        }

        NSMutableArray *cpuTemps = [NSMutableArray array];
        NSMutableArray *gpuTemps = [NSMutableArray array];
        NSMutableArray *allValidTemps = [NSMutableArray array];

        for (id service in services) {
            IOHIDServiceClientRef serviceRef = (__bridge IOHIDServiceClientRef)service;
            IOHIDEventRef event = IOHIDServiceClientCopyEvent(serviceRef, kIOHIDEventTypeTemperature, 0, 0);

            if (event) {
                double temp = IOHIDEventGetFloatValue(event, IOHIDEventFieldBase(kIOHIDEventTypeTemperature));

                // Only add valid temperatures (not 0 and reasonable range)
                if (temp > 0.0 && temp < 150.0) {
                    // Get product name to identify sensor type
                    NSString *productName = (__bridge NSString *)IOHIDServiceClientCopyProperty(
                        serviceRef, CFSTR("Product")
                    );

                    if (productName) {
                        NSString *productLower = [productName lowercaseString];

                        // Categorize based on product name patterns
                        // Stats uses these patterns for Apple Silicon:
                        // - pACC MTR Temp = Performance CPU cores
                        // - eACC MTR Temp = Efficiency CPU cores
                        // - GPU MTR Temp = GPU cores
                        // However, many Macs use PMU (Power Management Unit) sensors instead

                        BOOL isCPU = NO;
                        BOOL isGPU = NO;

                        // Check for explicit CPU/GPU markers
                        if ([productLower containsString:@"cpu"] ||
                            [productLower containsString:@"pacc mtr temp"] ||
                            [productLower containsString:@"eacc mtr temp"] ||
                            [productLower containsString:@"acc mtr temp"]) {
                            isCPU = YES;
                        } else if ([productLower containsString:@"gpu"] ||
                                   [productLower containsString:@"gpu mtr temp"]) {
                            isGPU = YES;
                        }
                        // PMU tdie sensors are usually CPU/SOC related (die temperature)
                        else if ([productLower containsString:@"pmu tdie"]) {
                            isCPU = YES;
                        }

                        if (isCPU) {
                            [cpuTemps addObject:@(temp)];
                        } else if (isGPU) {
                            [gpuTemps addObject:@(temp)];
                        }

                        // Collect all valid temps for fallback average
                        [allValidTemps addObject:@(temp)];

                        CFRelease((CFStringRef)productName);
                    } else {
                        // No product name, add to all valid temps
                        [allValidTemps addObject:@(temp)];
                    }
                }

                CFRelease(event);
            }
        }

        if (servicesRef) CFRelease(servicesRef);
        CFRelease(system);

        // Calculate average CPU temperature (from PMU tdie sensors)
        double cpuAvg = 0.0;
        double cpuMax = 0.0;
        if ([cpuTemps count] > 0) {
            double cpuSum = 0.0;
            for (NSNumber *temp in cpuTemps) {
                double val = [temp doubleValue];
                cpuSum += val;
                if (val > cpuMax) {
                    cpuMax = val;
                }
            }
            cpuAvg = cpuSum / [cpuTemps count];
        } else if ([allValidTemps count] > 0) {
            // Fallback: use average of all sensors if no CPU sensors identified
            double sum = 0.0;
            for (NSNumber *temp in allValidTemps) {
                double val = [temp doubleValue];
                sum += val;
                if (val > cpuMax) {
                    cpuMax = val;
                }
            }
            cpuAvg = sum / [allValidTemps count];
        }

        // Calculate average GPU temperature
        double gpuAvg = 0.0;
        double gpuMax = 0.0;
        if ([gpuTemps count] > 0) {
            double gpuSum = 0.0;
            for (NSNumber *temp in gpuTemps) {
                double val = [temp doubleValue];
                gpuSum += val;
                if (val > gpuMax) {
                    gpuMax = val;
                }
            }
            gpuAvg = gpuSum / [gpuTemps count];
        }
        // If no GPU sensors found, use the max temperature from CPU sensors as estimate
        // (on integrated graphics, GPU and CPU share the die)
        else if ([cpuTemps count] > 0) {
            gpuAvg = cpuMax;
        }

        // Return format: "cpuAvg,gpuAvg,cpuCount,gpuCount"
        NSString *result = [NSString stringWithFormat:@"%.1f,%.1f,%lu,%lu",
                           cpuAvg, gpuAvg,
                           (unsigned long)[cpuTemps count],
                           (unsigned long)[gpuTemps count]];
        return strdup([result UTF8String]);
    }
}
*/
import "C"
import (
	"fmt"
	"strconv"
	"strings"
	"unsafe"
)

// GetTemperaturesHID returns temperature information from HID sensors (for Apple Silicon)
func GetTemperaturesHID() (*TemperatureInfo, error) {
	cStr := C.getHIDTemperatures()
	if cStr == nil {
		return &TemperatureInfo{CPU: 0, GPU: 0}, fmt.Errorf("failed to get HID temperatures")
	}
	defer C.free(unsafe.Pointer(cStr))

	result := C.GoString(cStr)
	if result == "" {
		return &TemperatureInfo{CPU: 0, GPU: 0}, fmt.Errorf("no HID temperature sensors found")
	}

	// Parse result: "cpuAvg,gpuAvg,cpuCount,gpuCount"
	parts := strings.Split(result, ",")
	if len(parts) < 4 {
		return &TemperatureInfo{CPU: 0, GPU: 0}, fmt.Errorf("invalid temperature data format")
	}

	cpuTemp, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return &TemperatureInfo{CPU: 0, GPU: 0}, fmt.Errorf("failed to parse CPU temperature: %w", err)
	}

	gpuTemp, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return &TemperatureInfo{CPU: 0, GPU: 0}, fmt.Errorf("failed to parse GPU temperature: %w", err)
	}

	return &TemperatureInfo{
		CPU: cpuTemp,
		GPU: gpuTemp,
	}, nil
}
