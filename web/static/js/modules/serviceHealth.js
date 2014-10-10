/* global angular, console, $ */
/* jshint multistr: true */
(function() {
    'use strict';

    angular.module('serviceHealth', []).
    factory("$serviceHealth", ["$rootScope", "$q", "resourcesService", "$interval", "$translate",
    function($rootScope, $q, resourcesService, $interval, $translate){

        var servicesService = resourcesService,
            statuses = {};

        var STATUS_STYLES = {
            "bad": "glyphicon glyphicon-exclamation bad",
            "good": "glyphicon glyphicon-ok good",
            "unknown": "glyphicon glyphicon-question unknown",
            "down": "glyphicon glyphicon-minus disabled"
        };

        // simple array search util
        function findInArray(key, arr, val){
            for(var i = 0; i < arr.length; i++){
                if(arr[i][key] === val){
                    return arr[i];
                }
            }
        }

        // updates health check data for all services
        // `appId` is the id of the specific service being clicked
        function update(appId) {

            // TODO - these methods should return promises, but they
            // don't so use our own promises
            var servicesDeferred = $q.defer();
            var healthCheckDeferred = $q.defer();

            servicesService.update_services(function(top, mapped){
                servicesDeferred.resolve(mapped);
            });

            servicesService.get_service_health(function(healthChecks){
                healthCheckDeferred.resolve(healthChecks);
            });

            $q.all({
                services: servicesDeferred.promise,
                health: healthCheckDeferred.promise
            }).then(function(results){
                var serviceHealthCheck, instanceHealthCheck,
                    serviceStatus, instanceStatus, instanceUniqueId,
                    statuses = {};

                // iterate services healthchecks
                for(var serviceId in results.services){
                    serviceHealthCheck = results.health.Statuses[serviceId];
                    serviceStatus = new Status(serviceId, results.services[serviceId].Name, results.services[serviceId].DesiredState);

                    // if no healthcheck for this service, and this service 
                    // wasn't called out specifically with appId, mark as down
                    if(!serviceHealthCheck && serviceId !== appId){
                        serviceStatus.statusRollup.incDown();
                        serviceStatus.evaluateStatus();
                   
                    // fake "unknown" status for this service as it is
                    // probably on its way up right no
                    } else if(!serviceHealthCheck && appId === serviceId){
                        // TODO - cleaner way to set this
                        serviceStatus.statusRollup.incUnknown();
                        serviceStatus.evaluateStatus();

                    // otherwise, look for instances
                    } else {

                        // iterate instances healthchecks
                        for(var instanceId in serviceHealthCheck){
                            instanceHealthCheck = serviceHealthCheck[instanceId];
                            instanceUniqueId = serviceId +"."+ instanceId;
                            // evaluate the status of this instance
                            instanceStatus = new Status(instanceUniqueId, results.services[serviceId].Name +" "+ instanceId, results.services[serviceId].DesiredState);
                            instanceStatus.evaluateHealthChecks(instanceHealthCheck, results.health.Timestamp);
                            
                            // add this guy's statuses to hash map for easy lookup
                            statuses[instanceUniqueId] = instanceStatus;
                            // add this guy's status to his parent
                            serviceStatus.children.push(instanceStatus);
                        }
                        
                        // now that this services instances have been evaluated,
                        // evaluate the status of this service
                        serviceStatus.evaluateChildren();
                    }

                    statuses[serviceId] = serviceStatus;
                }

                updateHealthCheckUI(statuses);

            }).catch(function(err){
                // something went awry
                console.log("Promise err", err);
            });
        }

        // used by Status to examine children and figure
        // out what the parent's status is
        function StatusRollup(){
            this.good = 0;
            this.bad = 0;
            this.down = 0;
            this.unknown = 0;
            this.total = 0;
        }
        StatusRollup.prototype = {
            constructor: StatusRollup,

            incGood: function(){
                this.good++;
                this.total++;
            },
            incBad: function(){
                this.bad++;
                this.total++;
            },
            incDown: function(){
                this.down++;
                this.total++;
            },
            incUnknown: function(){
                this.unknown++;
                this.total++;
            },

            // TODO - use assertion style ie: status.is.good() or status.any.good()
            anyBad: function(){
                return !!this.bad;
            },
            allBad: function(){
                return this.total && this.bad === this.total;
            },
            anyGood: function(){
                return !!this.good;
            },
            allGood: function(){
                return this.total && this.good === this.total;
            },
            anyDown: function(){
                return !!this.down;
            },
            allDown: function(){
                return this.total && this.down === this.total;
            },
            anyUnknown: function(){
                return !!this.unknown;
            },
            allUnknown: function(){
                return this.total && this.unknown === this.total;
            }
        };

        function Status(id, name, desiredState){
            this.id = id;
            this.name = name;
            this.desiredState = desiredState;

            this.statusRollup = new StatusRollup();
            this.children = [];

            // bad, good, unknown, down
            // TODO - enum?
            this.status = null;
            this.description = null;
        }

        Status.prototype = {
            constructor: Status,

            // distill this service's statusRollup into a single value
            evaluateStatus: function(){
                if(this.desiredState === 1){
                    // if any failing, bad!
                    if(this.statusRollup.anyBad()){
                        this.status = "bad";
                        this.description = $translate.instant("failing_health_checks");

                    // if any down, oh no!
                    } else if(this.statusRollup.anyDown()){
                        this.status = "unknown";
                        this.description = $translate.instant("container_unavailable");

                    // if all are good, yay! good!
                    } else if(this.statusRollup.allGood()){
                        this.status = "good";
                        this.description = $translate.instant("passing_health_checks");
                    
                    // huh. no idea what happened.
                    } else {
                        this.status = "unknown";
                        this.description = "huh... what had happen?";
                    }

                } else if(this.desiredState === 0){
                    // if everyone is down, yay!
                    if(this.statusRollup.allDown()){
                        this.status = "down";
                        this.description = $translate.instant("container_down");

                    // stuff is down as expected
                    } else {
                        this.status = "unknown";
                        this.description = $translate.instant("stopping_container");
                    }
                }
            },

            // roll up child status into this status
            evaluateChildren: function(){

                // TODO - handle no children

                this.statusRollup = this.children.reduce(function(acc, childStatus){
                    // TODO - don't directly access property
                    acc[childStatus.status]++;
                    acc.total++;

                    return acc;
                }.bind(this), new StatusRollup());

                // TODO - pass description up through this stuff
                this.evaluateStatus();
            },

            // set this status's statusRollup based on healthchecks
            evaluateHealthChecks: function(healthChecks, timestamp){
                var status;

                this.statusRollup = new StatusRollup();

                for(var healthCheck in healthChecks){
                    status = evaluateHealthCheck(healthChecks[healthCheck], timestamp);

                    // this is a healthcheck status object... kinda weird...
                    this.children.push({
                        name: healthCheck,
                        status: status
                    });
                    
                    // add this guy's status to the total
                    // TODO - use method, don't directly access property!
                    this.statusRollup[status]++;
                    this.statusRollup.total++;
                }

                this.evaluateStatus();
            },

        };

        // determine the health of a healthCheck based on start time, 
        // up time and healthcheck
        function evaluateHealthCheck(hc, timestamp){
            var status = {};

            // calculates the number of missed healthchecks since last start time
            var missedIntervals = (timestamp - Math.max(hc.Timestamp, hc.StartedAt)) / hc.Interval;

            // if service hasn't started yet
            if(hc.StartedAt === undefined){
                status = "down";
            
            // if healthCheck has missed 2 updates, mark unknown
            } else if (missedIntervals > 2 && missedIntervals < 60) {
                status = "unknown";

            // if healthCheck has missed 60 updates, mark failed
            } else if (missedIntervals > 60) {
                status = "bad";

            // if Status is passed, then good!
            } else if(hc.Status === "passed") {
                status = "good";

            // if Status is failed, then bad!
            } else if(hc.Status === "failed") {
                status = "bad";

            // otherwise I have no idea
            } else {
                status = "unknown";
            }

            return status;
        }

        var healthcheckTemplate = '<div class="healthIconBG"></div><i class="healthIcon glyphicon"></i><div class="healthIconBadge"></div>';

        function updateHealthCheckUI(statuses){
            // select all healthchecks DOM elements and look
            // up their respective class thing
            $(".healthCheck").each(function(i, el){
                var $el = $(el),
                    id = el.dataset.id,
                    lastStatus = el.dataset.lastStatus,
                    $healthIcon, $badge,
                    statusObj, popoverHTML;


                // if this is an unintialized healthcheck html element,
                // put template stuff inside
                if(!$el.children().length){
                    $el.html(healthcheckTemplate);
                }

                $healthIcon = $el.find(".healthIcon");
                $badge = $el.find(".healthIconBadge");

                // for some reason this healthcheck has no id,
                // so no icon for you!
                if(!id){
                    return;
                }

                statusObj = statuses[id];

                // TODO - this probably means the container is coming up,
                // so handle this case
                if(!statusObj){
                    console.log("no statusObj for", id);
                    return;
                }

                // if there is more than one instance, and they are not
                // all in a good state, show a status count
                if(!statusObj.statusRollup.allGood() && !statusObj.statusRollup.allDown()){
                    $el.addClass("wide");
                    $badge.text(statusObj.statusRollup[statusObj.status] +"/"+ statusObj.statusRollup.total);
                    $badge.show();

                // else, hide the badge
                } else {
                    $el.removeClass("wide");
                    $badge.hide();
                }
               
                // setup popover
                // remove any existing popover if not currently visible
                if($el.popover && !$el.next('div.popover:visible').length){
                    $el.popover('destroy');
                }

                // if this statusObj has children, we wanna show
                // them in the healtcheck tooltip, so generate
                // some yummy html
                if(statusObj.children.length){
                    popoverHTML = [];

                    // TODO - MOVE THIS FUNCTION! GAH!
                    var hcRowTemplate = function(hc){
                        return "<div class='healthTooltipDetailRow "+ hc.status +"'>\
                                <i class='healthIcon glyphicon'></i>\
                            <div class='healthTooltipDetailName'>"+ hc.name +"</div>\
                        </div>";
                    };

                    var isHealthCheckStatus = function(status){
                       return !status.id;
                    };

                    // if this status's children are healthchecks,
                    // no need for instance rows, go straight to healthcheck rows
                    if(statusObj.children.length && isHealthCheckStatus(statusObj.children[0])){
                        statusObj.children.forEach(function(hc){
                            popoverHTML.push(hcRowTemplate(hc));
                        });
                         
                    // else these are instances, so create instance rows
                    // AND healthcheck rows
                    } else {
                        statusObj.children.forEach(function(instanceStatus){
                            popoverHTML.push("<div class='healthTooltipDetailRow'>");
                            popoverHTML.push("<div style='font-weight: bold; font-size: .9em; padding: 5px 0 3px 0;'>"+ instanceStatus.name +"</div>");
                            instanceStatus.children.forEach(function(hc){
                                popoverHTML.push(hcRowTemplate(hc));
                            });
                            popoverHTML.push("</div>");
                        });
                    }

                    popoverHTML = popoverHTML.join("");
                }

                // configure popover
                // TODO - dont touch dom!
                $el.popover({
                    trigger: "hover",
                    placement: "right",
                    delay: 0,
                    title: statusObj.description,
                    html: true,
                    content: popoverHTML,

                    // if DesiredState is 0 or there are no healthchecks, the
                    // popover should be only a title with no content
                    template: statusObj.desiredState === 0 || !popoverHTML ?
                        '<div class="popover" role="tooltip"><div class="arrow"></div><h3 class="popover-title"></h3></div>' :
                        undefined
                });

                $el.removeClass(Object.keys(STATUS_STYLES).join(" "))
                    .addClass(statusObj.status);

                // if the status has changed since last tick, or
                // it was and is still unknown, notify user
                if(lastStatus !== statusObj.status || lastStatus === "unknown" && statusObj.status === "unknown"){
                    bounceStatus($el);
                }
                // store the status for comparison later
                el.dataset.lastStatus = statusObj.status;
            });
        }

       function bounceStatus($el){
            $el.addClass("zoom");

            $el.on("webkitAnimationEnd mozAnimationEnd", function(){
                $el.removeClass("zoom");
                // clean up animation end listener
                $el.off("webkitAnimationEnd mozAnimationEnd");
            });
        }

        return {
            update: update
        };
    }]);

})();
