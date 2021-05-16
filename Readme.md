### Code Test 1 - Julian Alwandy
To run this code, you can use the Makefile to build the project which will build it to `build/application` and then run it `./build/application` you might need to do `chmod 777 build/application` to run it 

### Notes 
Test sample is incorrect as it contains successfull connection during the ratelimit window for the IPs which it shouldn't or at least contain 429 status codes. Just a note, it's over 90k lines so going through it is too long to verify every single one. 