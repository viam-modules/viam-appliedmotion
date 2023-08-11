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
