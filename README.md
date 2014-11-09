transit_tools
=============
Tools for processing transit agency data.
* Original author: James Synge, Lexington, MA
* On the web here: http://enigma2eureka.blogspot.com/
* G+: https://plus.google.com/+JamesSynge

Initially this contains the following:

* **nextbus_fetcher.go** Continuously fetches the vehicle location
  data for an agency from NextBus, archiving it in both raw form 
  (the XML responses from NextBus), and in processed form (CSV
  files for each data, with each unique vehicle location report.
* **nextbus/...** Utilities for processing messages from NextBus,
  including for modelling the info from a transit agency, and
  especially for manipulating the geographic information related
  to bus routes (i.e. do the vehicle location reports match up
  with the routes defined by the agency).
* **geom/...** 2-d planar geometry code, in support of processing
  vehicle locations and bus routes in a metropolitan area (i.e.
  in a region we can approximate as planar).
* **geo/...** Geographic code, for dealing with positions, headings
  and distances, and for converting those from earth coordinates
  to planar metropolitan coordinates.
* **fit/...** Line fitting code, for adding in computing new path
  segments for a route based on the actual path taken by buses
  (not really necessary for trains ;-)).
* **stats/...** 1-d and 2d statistics code, in support of the line
  fitting.
* **util/...** Lots of utility code, such as rate limiting for
  HTTP fetches, tools for reading and writing CSV files, creating
  compressed TAR files, including rolling archives, pretty printing
  timestamps.


