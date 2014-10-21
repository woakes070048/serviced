/* global angular, console, $ */
/* jshint multistr: true */

/*
 * servicesService
 *
 * provides an interface for obtaining services in various ways.
 * This should supercede resourcesService service handling due to
 * the difficulty keeping the services up to date once a controller
 * grabs them.
 *
 * Note that the service objects used throughout should always
 * be the same unique object. If any change is made, the existing
 * object should be modified rather than creating a new object. This
 * makes it easy for controllers who are bound to them objects to
 * be updated without any extra work on their part
 *
 * TODO - make an actual ServiceObject prototype
 */

(function() {
    'use strict';

        // services arrange hierarchically
    var servicesTree = [],
    
        // all services, flattened, keyed by id
        servicesMap = {},

        // time of last update
        lastUpdate;


    angular.module('servicesService', []).
    factory("servicesService", ["resourcesService", "$http", "$q",
    function(resourcesService, $http, $q){

        // updates existing servicesTree and servicesMap
        var update = function(){

            // TODO - resourcesService should return promises
            var deferred = $q.defer();

            resourcesService.get_services(true, function(tree, map){
                // update the flat map of services, then
                // generate a tree structure from the mapped services
                buildTree(buildMap(map, servicesMap), servicesTree);

                // TODO - vhosts, ips, etc
                
                deferred.resolve(map);
            });

            return deferred.promise;

        };

        // keep services up to date by polling
        // TODO - websockets!!!!!!!!1!!!!11!!!
        //resourcesService.registerPoll("servicesService", function(){
            //var since,
                //url = "/services";

            //if(lastUpdate){
                //// calculate time in ms since last update
                //since = new Date().getTime() - lastUpdate;
                //// add one second buffer to be sure we don't miss anything
                //since += 1000;

                //url += "?since=" + since;
            //}


            //// TODO - use resourcesService here to get the data!
            //$http.get(url).then(function(data){
                //// store the update time for comparison later
                //lastUpdate = new Date().getTime();

                //update(data.data);
            //});
        

        //}, 3000);

        // public interface
        return {
            getTree: getTree,
            getMap: getMap,
            update: update
        };

    }]);

    // iterates flat services map and builds a tree
    // based on ParentServiceID
    function buildTree(services, baseTree){
        var serviceTree = baseTree || [],
            service, parent;

        for(var id in services){
            service = services[id];

            // if this has a parent, find the parent
            // and attach self
            if(service.ParentServiceID){
                // TODO - handle missing parent
                parent = servicesMap[service.ParentServiceID];

                parent.children = parent.children || [];
                
                findAndMergeServiceObject(parent.children, service);

            // if no parent, this is a top level service
            // and gets to sit at the big boy table
            } else {
                findAndMergeServiceObject(serviceTree, service);
            }
        }

        return serviceTree;
    }

    // iterates array of services and creates a mapping
    // of service id to service object
    // if baseMap is provided, changes are made to baseMap.
    // otherwise, changes are made to a new object
    function buildMap(services, baseMap){
        var serviceMap = baseMap || {};

        for(var i in services){
            serviceMap[services[i].ID] = serviceMap[services[i].ID] || {};
            
            // update with the new service keys rather than
            // replacing it with the new service object
            mergeServiceObject(serviceMap[services[i].ID], services[i]);
        }

        return serviceMap;
    }

    // takes an old service object and merges new service
    // object on top of it, preserving the identity of
    // the old object, while updating the properties
    function mergeServiceObject(oldService, newService){
        return angular.extend(oldService || {}, newService);
    }

    // searches array `children` for a child with ID that
    // matches service `service`'s ID property and merges
    // the services together. If not found, the service is
    // pushed into `children`
    function findAndMergeServiceObject(children, service){
        var found = false;

        for(var i = 0; i < children.length - 1; i++){
            if(children[i].ID === service.ID){
                mergeServiceObject(children[i], service);
                found = true;
                break;
            }
        }

        if(!found) children.push(service);
    }

    // returns service, with all descendents, in tree form
    // if no id, returns all services
    function getTree(id){
        // if no id, return all services
        if(id === undefined){
            return servicesTree;
        
        // return specific service (and children)
        } else {
            return servicesMap[id];
        }
    }

    // returns service, with all descendents, in a flat
    // map, keyed by service id
    // if no id, returns all services
    // TODO - memoize
    function getMap(id){
        // TODO - if no id, getTree will return an array
        // of services, which won't fit into the recursion
        // pattern properly. They need to be children of
        // something
        var service = getTree(id),
            map = {};

        (function recurse(subservice, depth){
            // hmm... this shouldn't happen :/
            if(!subservice && !subservice.ID) return;

            // add this service to the list
            map[subservice.ID] = subservice;

            // store this service's tree depth
            subservice.zendepth = depth;

            // if there are children, recursively iterate
            if(subservice.children && subservice.children.length){
                subservice.children.forEach(recurse, depth+1);
            }
        })(service, 0);

        // TODO - the objects in this map are linked correctly
        // but the map itself is not, meaning adding or removing
        // services wont be reflected in this map
        return map;
    }

})();
