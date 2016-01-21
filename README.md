# cs733
##Assignment 1
File Server in go <br>
###Commands :
1. Write: create a file, or update the file’s contents if it already exists.<br>
Usage : 
write \<filename\> \<numbytes\> [\<exptime\>]\r\n\<content bytes\>\r\n<br>
Response :
OK \<version\>\r\n
2. Read: Given a filename, retrieve the corresponding file.<br>
Usage: read \<filename\>\r\n<br>
Response :
CONTENTS \<version\> \<numbytes\> \<exptime\>\r\n
\<content bytes\>\r\n
3. Compare and swap. This replaces the old file contents with the new content
provided the version is still the same.<br>
Usage: 
cas \<filename\> \<version\> \<numbytes\> [\<exptime\>]\r\n
\<content bytes\>\r\n<br>
Response :
OK \<version\>\r\n
4. Delete file<br>
Usage: delete \<filename\>\r\n<br>
Response: 
OK\r\n

###Errors :
1. ERR_VERSION\r\n (the contents were not updated because of a version
mismatch)
2. ERR_FILE_NOT_FOUND\r\n (the filename doesn’t exist)
3. ERR_CMD_ERR\r\n (the command is not formatted correctly)
4. ERR_INTERNAL\r\n (any other error you wish to report that is not covered by the
rest)

###Bugs :
1. If the power fails, then all the data is lost

