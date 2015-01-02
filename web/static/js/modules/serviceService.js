// serviceService
// maintains a list of services and fills out children,
// hosts, ips, etc
(function() {
    'use strict';

    // services in tree form
    var serviceTree = [],
    // service dictionary keyed by id
        serviceMap = {};

    angular.module('servicesService', []).
    factory("$servicesService", ["$rootScope", "$q", "resourcesService", "$interval", "$serviceHealth",
    function($rootScope, $q, resourcesService, $interval, serviceHealth){

        init();

        return {
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
            init: init,
            // get by name
            get: function(name){
                for(var id in serviceMap){
                    if(serviceMap[id].name === name){
                        return serviceMap[id]; 
                    } 
                } 
            }
        };


        // hits the services endpoint and asks
        // for changes since refresh, then rebuilds
        // serviceTree
        // TODO - update list by application instead
        // of all services ever?
        function update(){
            // TODO - use resourcesService to make requests
            initPromise.then(function(){
                $.get("/services?since=4000&testy='testy'")
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
                // HACK - services should update themselves
                serviceHealth.update(serviceMap);
            });
        }

        var initPromise;

        // makes the initial services request
        function init(){

            // if init hasnt been called, create a new promise
            // and make the initial request for services
            if(!initPromise){
                // TODO - use resourcesService to make requests
                initPromise = $.get('/services')
                    .success(function(data, status) {

                        var service, parent;

                        // store services as a flat map
                        data.forEach(function(service){
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

            return initPromise;
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
                serviceTree.sort(sortServicesByName);
            }

            // ICKY GROSS HACK!
            // iterate tree and store tree depth on
            // individual services
            // TODO - find a more elegant way to keep track of depth
            serviceTree.forEach(function(topService){
                topService.depth = 0;
                topService.children.forEach(function recurse(service){
                    service.depth = service.parent.depth + 1;
                    service.children.forEach(recurse);
                });
            });
        }

    }]);

    function sortServicesByName(a, b){
        return a.name.toLowerCase() < b.name.toLowerCase() ? -1 : 1;
    }

    // Service object constructor
    // takes a service object (backend service object)
    // and wraps it with extra functionality and info
    function Service(service, parent){
        console.log("Creating service", service.Name);
        this.parent = parent;
        this.children = [];

        // tree depth
        this.depth = 0;

        // cache for computed values
        this.cache = new Cache(["vhosts", "addresses", "descendents", "instances"]);

        this.update(service);

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
            // make service immutable
            this.service = Object.freeze(service);
            
            // these properties are for convenience
            this.name = service.Name;
            this.id = service.ID;

            // TODO - update service health

            // infer service type
            // TODO - check for more types
            // TODO - set a type property 
            if(this.service.ID.indexOf("isvc-") != -1){
                this.isvc = true;
            }

            // invalidate caches
            this.markDirty();

            console.log("Updating service", this.service.Name);
        },

        // invalidate all caches. This is needed 
        // when descendents update
        markDirty: function(){
            this.cache.markAllDirty();
            console.log("Invalidating all caches", this.service.Name);
        },

        addChild: function(service){
            // TODO - check for duplicates?
            this.children.push(service);

            this.markDirty();

            // alpha sort children
            this.children.sort(sortServicesByName);
        },

        removeChild: function(service){
            // TODO - remove child
            this.markDirty();
        },

        addParent: function(service){
            if(this.parent){
                this.parent.removeChild(this);
            }
            this.parent = service;
            this.markDirty();
        },

        // returns a list of VHosts for
        // service and all children, and
        // caches the list
        aggregateVHosts: function(){
            var hosts = this.cache.getIfClean("vhosts");

            // if valid cache, early return it
            if(hosts){
                console.log("Using cached vhosts for ", this.name);
                return hosts;
            }

            console.log("Calculating vhosts for ", this.name);
            // otherwise, get some data
            var services = this.aggregateDescendents().slice();

            // we also want to see the Endpoints for this
            // service, so add it to the list
            services.push(this);

            // iterate services
            hosts = services.reduce(function(acc, service){

                var result = [];

                // if Endpoints, iterate Endpoints
                if(service.service.Endpoints){
                    result = service.service.Endpoints.reduce(function(acc, endpoint){
                        // if VHosts, iterate VHosts
                        if(endpoint.VHosts){
                            endpoint.VHosts.forEach(function(VHost){
                                acc.push({
                                    Name: VHost,
                                    Application: service.name,
                                    ServiceEndpoint: endpoint.Application,
                                    ApplicationId: service.id
                                });
                            });
                        }

                        return acc;
                    }, []);
                }

                return acc.concat(result);
            }, []);

            this.cache.cache("vhosts", hosts);
            return hosts;
        },

        // returns a list of address assignments
        // for service and all children, and
        // caches the list
        aggregateAddressAssignments: function(){

            var addresses = this.cache.getIfClean("addresses");

            // if valid cache, early return it
            if(addresses){
                console.log("Using cached addresses for ", this.name);
                return addresses;
            }

            console.log("Calculating addresses for ", this.name);
            // otherwise, get some new data
            var services = this.aggregateDescendents().slice();

            // we also want to see the Endpoints for this
            // service, so add it to the list
            services.push(this);

            // iterate services
            addresses = services.reduce(function(acc, service){

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
                                ServiceID: service.id,
                                ServiceName: service.name
                            });
                        }
                        return acc;
                    }, []);
                }

                return acc.concat(result);
            }, []);

            this.cache.cache("addresses", addresses);
            return addresses;

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
            var deferred = $q.defer(),
                cachedInstances = this.cache.getIfClean("instances");

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


})();
