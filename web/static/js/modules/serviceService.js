// serviceService
// maintains a list of services and fills out children,
// hosts, ips, etc
(function(){
// Service object constructor
// takes a service object (backend service object)
// and wraps it with extra functionality and info
function Service(service, parent){
    console.log("Creating service", service.Name);
    this.parent = parent;
    this.children = [];

    // cache for computed values
    this.cache = new Cache(["vhosts", "addresses", "descendents", "instances"]);

    this.update(service);

    // these properties are for convenience
    this.name = service.Name;

    // these properties are for convenience
    this.name = service.Name;
    this.id = service.ID;

    // this newly created child should be
    // registered with its parent
    // TODO - this makes parent update twice...
    if(this.parent){
        this.parent.addChild(this);
    }
}

Service.prototype = {
    constructor: Service,

    // updates the immutable service object
    // and marks any computed properties dirty
    update: function(service){
        // if a new service is provided,
        // replace the existing service
        if(service){
            // make service immutable
            this.service = Object.freeze(service);
        }

        // TODO - update service health
        // TODO - infer service type: isvcs,
        // container(children but no startup), regular
        this.cache.markAllDirty();

        console.log("Updating service", this.service.Name);

        // notify parent of update
        if(this.parent){
            this.parent.update();
        }
    },

    addChild: function(service){
        // TODO - check for duplicates?
        this.children.push(service);
        this.update();
    },

    removeChild: function(service){
        // TODO - remove child
        this.update();
    },

    addParent: function(service){
        if(this.parent){
            this.parent.removeChild(this);
        }
        this.parent = service;
        this.update();
    },

    // returns a list of VHosts for
    // service and all children, and
    // caches the list
    aggregateVHosts: function(){
        var services = this.aggregateDescendents(),
            top = this.service;

        // we also want to see the Endpoints for this
        // service, so add it to the list
        services.push(this);

        // iterate services
        return services.reduce(function(acc, child){

            var result = [];

            // if Endpoints, iterate Endpoints
            if(child.service.Endpoints){
                result = child.service.Endpoints.reduce(function(acc, endpoint){
                    // if VHosts, iterate VHosts
                    if(endpoint.VHosts){
                        endpoint.VHosts.forEach(function(VHost){
                            acc.push({
                                Name: VHost,
                                Application: top.Name,
                                ServiceEndpoint: endpoint.Application,
                                ApplicationId: top.ID
                            });
                        });
                    }

                    return acc;
                }, []);
            }

            return acc.concat(result);
        }, []);
    },

    // returns a list of address assignments
    // for service and all children, and
    // caches the list
    aggregateAddressAssignments: function(){
        var services = this.aggregateDescendents(),
            top = this.service;

        // we also want to see the Endpoints for this
        // service, so add it to the list
        services.push(this);

        // iterate services
        return services.reduce(function(acc, service){

            var result = [];

            // if Endpoints, iterate Endpoints
            if(service.service.Endpoints){
                result = service.service.Endpoints.reduce(function(acc, endpoint){
                    if (endpoint.AddressConfig.Port > 0 && endpoint.AddressConfig.Protocol) {
                        acc.push({
                            ID: endpoint.AddressAssignment.ID,
                            AssignmentType: endpoint.AddressAssignment.AssignmentType,
                            EndpointName: endpoint.AddressAssignment.EndpointName,
                            HostID: endpoint.AddressAssignment.HostID,
                            // TODO - lookup host name from resourcesService.get_host
                            HostName: "unknown",
                            PoolID: endpoint.AddressAssignment.PoolID,
                            IPAddr: endpoint.AddressAssignment.IPAddr,
                            Port: endpoint.AddressConfig.Port,
                            ServiceID: top.ID,
                            ServiceName: top.Name
                        });
                    }
                    return acc;
                }, []);
            }

            return acc.concat(result);
        }, []);
    },

    // returns a flat map of all descendents
    // of this service
    aggregateDescendents: function(){
        var descendents = this.cache.getIfClean("descendents");

        if(descendents){
            console.log("Using cached descendents for", this.name);
            return descendents;
        } else {
            console.log("Calculating descendents for", this.name);
            descendents = this.children.reduce(function(acc, child){
                acc.push(child);
                return acc.concat(child.aggregateDescendents());
            }, []);

            this.cache.cache("descendents", descendents);
            return descendents;
        }

    },

    // returns a promise good for a list
    // of running service instances
    getInstances: function(){
        // TODO - use legit promise lib
        // NOTE: i know this isnt how promises work! shut up!
        var deferred = {
            state: "pending",
            thens: [],
            resolve: function(val){
                this.val = val;
                this.state = "resolved";
                this.thens.forEach(function(fn){ fn(val); });
            },
            then: function(fn){
                if(this.state == "resolved"){
                    fn(this.val);
                } else {
                    this.thens.push(fn);
                }
            }
        };

        var cachedInstances = this.cache.getIfClean("instances");

        // if instances is cached and not dirty,
        // return those
        if(cachedInstances){
            console.log("Resolving cached instances for", this.name);
            deferred.resolve(cachedInstances);

        // otherwise, refresh instances
        } else {
            // TODO - use resourcesService to make requests
            $.get("/services/"+ this.id +"/running")
                .success(function(data, status) {
                    // TODO - generate Instance objects?
                    this.cache.cache("instances", data);
                    console.log("Retrieving instances for", this.name);
                    deferred.resolve(data);
                }.bind(this));
        }

        return deferred;

    },
};



// simple cache object
// TODO - angular has this sorta stuff built in
function Cache(caches){
    this.caches = {};
    if(caches){
        caches.forEach(function(name){
            this.addCache(name);
        }.bind(this));
    }
}
Cache.prototype = {
    constructor: Cache,
    addCache: function(name){
        this.caches[name] = {
            data: null,
            dirty: false
        };
    },
    markDirty: function(name){
        this.mark(name, true);
    },
    markAllDirty: function(){
        for(var key in this.caches){
            this.markDirty(key);
        }
    },
    markClean: function(name){
        this.mark(name, false);
    },
    markAllClean: function(){
        for(var key in this.caches){
            this.markClean(key);
        }
    },
    cache: function(name, data){
        this.caches[name].data = data;
        this.caches[name].dirty = false;
    },
    get: function(name){
        return this.caches[name].data;
    },
    getIfClean: function(name){
        if(!this.caches[name].dirty){
            return this.caches[name].data;
        }
    },
    mark: function(name, flag){
        this.caches[name].dirty = flag; 
    },
    isDirty: function(name){
        return this.caches[name].dirty;
    }
};



// services in tree form
var serviceTree = [],
// service dictionary keyed by id
    serviceMap = {};

var serviceService = {
    // returns a service by id
    getService: function(id){
        return serviceMap[id];
    },

    // forces updateServiceList in case
    // a service wants to see changes asap!
    update: update,

    // NOTE - these are debug only! remove!
    serviceTree: serviceTree,
    serviceMap: serviceMap,
    init: init
};

// NOTE - this is debug only. remove!
window.s = serviceService;

// hits the services endpoint and asks
// for changes since refresh, then rebuilds
// serviceTree
// TODO - update list by application instead
// of all services ever?
function update(){
    // TODO - ensure init has been called
    // TODO - use resourcesService to make requests
    $.get("/services?since=4000")
        .success(function(data, status){
            // TODO - change backend to send
            // updated, created, and deleted
            // separately from each other
            data.forEach(function(service){
                // update
                if(serviceMap[service.ID]){
                    serviceMap[service.ID].update(service);
                
                // new
                } else {
                    serviceMap[service.ID] = new Service(service);
                    addServiceToTree(serviceMap[service.ID]);
                }

                // TODO - deleted serviced
            });
        });
}


// makes the initial services request
function init(){

    // TODO - use resourcesService to make requests
    $.get('/services')
        .success(function(data, status) {

            var service, parent;

            // store services as a flat map
            data.forEach(function(service){
                // flag internal services
                // TODO - move this to Service object
                service.isvc = service.ID.indexOf("isvc-") != -1;

                serviceMap[service.ID] = new Service(service);
            });

            // generate service tree
            for(var serviceId in serviceMap){
                // TODO - check for service
                service = serviceMap[serviceId];
                addServiceToTree(service);
            }
      });

      setInterval(update, 3000);
}

// adds a service object to the service tree
// in the appropriate place
function addServiceToTree(service){

    // if this is not a top level service
    if(service.service.ParentServiceID){
        // TODO - check for parent
        parent = serviceMap[service.service.ParentServiceID];
        // TODO - consider order here? adding child updates
        // then adding parent updates again
        parent.addChild(service);
        service.addParent(parent);

    // if this is a top level service
    } else {
        serviceTree.push(service);
    }
}

})();
