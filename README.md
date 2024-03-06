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
| acceleration* | float64 | The default acceleration rate to use for the start of move commands |
| deceleration* | float64 | The default deceleration rate to use for the end of move commands and explicit stop commands |
| min_acceleration* | float64 | The minimum acceleration rate to use for the start of move commands. Set this to 0 to not enforce any minimum value. |
| max_acceleration* | float64 | The maximum acceleration rate to use for the start of move commands. Set this to 0 to not enforce any maximum value. |
| min_deceleration* | float64 | The minimum deceleration rate to use for the end of move commands and explicit stop commands. Set this to 0 to not enforce any minimum value. |
| max_deceleration* | float64 | The maximum deceleration rate to use for the end of move commands and explicit stop commands. Set this to 0 to not enforce any maximum value. |

_*Denotes configuration value is optional_

## Network Setup (for ethernet-connected motor controllers like the STF10-IP)

Assuming you set the dial on the side of the controller to a static IP address such as 10.10.10.10, you will need to configure your computer to know where to find it. On your computer, go to the settings for the ethernet device to which the motor controller is connected. In the IPv4 settings, set the method for obtaining an IP address to Manual (rather than, for example, DHCP). Add the address 10.10.10.1 (note that this is _not_ the IP address of the motor! This is the IP address your computer should call itself when talking to the motor) with a netmask of 24, which instructs the computer to look for all addresses in the 10.10.10.xx subnet on that ethernet port, while still looking for all other traffic on other network connections. Save and close these settings.

## Using extra parameters in the movement RPCs

In the `GoTo` and `GoFor` commands, you can optionally set the `"acceleration"` and `"deceleration"` in the `extra` parameters. If you set either (or both!) of them, they should be 64-bit floating point numbers (sometimes called doubles), describing the acceleration/deceleration to use in revolutions per second^2. If you have also set the minimum or maximum acceleration/deceleration in the config, and the `extra` value falls outside the allowable range, we will instead use the minimum or maximum (depending on whether the `extra` value was too low or too high, respectively).

## Unspecified parameters

Any parameters not explicitly set (e.g., if you don't specify the acceleration, or you're interested in the torque ripple threshold which we don't support at all) will use whatever value was previously stored on the motor controller. This means you can use `DoCommand` to send raw values to the motor controller for all the extra parts you're interested in, and they will be respected by later movement commands.

For a description of the raw instructions you could send to the motor using `DoCommand`, see [the manual](https://appliedmotion.s3.amazonaws.com/Host-Command-Reference_920-0002W_0.pdf) for this hardware.

We will take care of the extra formatting: just put a `"AC100"` or similar in the `"command"` key of a `DoCommand`, without the null byte or bell at the beginning and without the carriage return at the end. We'll send back the result in the `"response"` key, again with the formatting bytes stripped out.
