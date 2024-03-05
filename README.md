# Applied Motion Products Modular Component for Viam

This modular component allows Viam to control the Applied Motion Products line of motor drivers. 

Support Matrix:
| Driver | Support |
| ------ | ------- |
| STF06-R | :warning: | 
| STF06-C | :no_entry_sign: | 
| STF06-D | :warning: | 
| STF06-IP | :warning: |
| STF06-EC | :no_entry_sign: | 
| STF10-R | :warning: |
| STF10-C | :no_entry_sign: | 
| STF10-D | :warning: | 
| STF10-IP | :white_check_mark: | 
| STF10-EC | :no_entry_sign: | 

:white_check_mark: - Known to work
:warning: - This has not been explicitly tested, but should, in theory, work.
:no_entry_sign: - Known to not work

Support has been explicity tested on the STF10-IP, and support for RS485 has been added (in as much as a path to a `/dev/xxxx` is accepted). This means that, while STF06 support has not been explicitly tested, it should work based on the Applied Motion Products documentation.

## Configuration
| Variable | DataType | Notes |
| -------- | -------- | ----- |
| protocol | string   | The protocol to use for communicating with the controller. Acceptable values are `ip`, `rs485`, and `rs232` |
| uri      | string   | Either the IP address or the path to the `rs232`/`rs485` interface on linux |
| min_rpm  | float64  | The minimum RPM that this motor can run |
| max_rpm  | float64  | The maximum RPM that this motor can run |
| steps_per_rev | int64 | The number of pulses required to drive the motor one revolution. This is configured in the drive using the Applied Motion software |
| connect_timeout* | int64 | The number of seconds to wait for the drive to respond |
| acceleration* | float64 | The acceleration rate to use for the start of move commands |
| deceleration* | float64 | The acceleration rate to use for the end of move commands and explicit stop commands |

_*Denotes configuration value is optional_

## Network Setup (for ethernet-connected motor controllers like the STF10-IP)

Assuming you set the dial on the side of the controller to a static IP address such as 10.10.10.10, you will need to configure your computer to know where to find it. On your computer, go to the settings for the ethernet device to which the motor controller is connected. In the IPv4 settings, set the method for obtaining an IP address to Manual (rather than, for example, DHCP). Add the address 10.10.10.1 with a netmask of 24, which instructs the computer to look for all addresses in the 10.10.10.xx on that ethernet port, while still looking for all other traffic on other network connections. Save and close these settings.
