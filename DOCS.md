# Api core

Focus on composability and ease of 

## Resources

Resources are the data part of the API, a resource can be defined as a package with a collection of models and controllers.
Resources should register their models and controllers but shouldn't override other controllers, routes and other intializations.
Resources should be considered a stand-alone package with fewest relations to others possible.
