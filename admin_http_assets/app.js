var app = new angular.module("carbon-relay-ng", ["ngResource", "ui.bootstrap"]);

app.controller("MainCtl", ["$scope", "$resource", "$modal", function($scope, $resource, $modal){
  $scope.alerts = [];
  var Table = $resource("/table/");
  var Blacklist = $resource("/blacklists/:index");
  var Aggregator = $resource("/aggregators/:index");
  var Route = $resource("/routes/:key", {key: '@key'}, {});
  var Destination = $resource("/routes/:key/destinations/:index");
  

  $scope.validAddress = /^[^:]+\:[0-9]+$/;
  $scope.validRegex = (function() {
      return {
          test: function(value) {
              var isValid = true;
              try { 
                new RegExp(value);
              } catch(e) {
                isValid = false;
              }
              return isValid;
          }
      };
  })();

  $scope.validAggFunc = (function() {
      return {
          test: function(value) {
              if (value == "sum") {
                  return true;
              }
              if (value == "avg") {
                  return true;
              }
              return false;
          }
      };
  })();

  $scope.list = function(idx){
    Table.get(function(data){
      $scope.table = data;
    });
  };

  $scope.list();

  $scope.add = function() {
    $scope.alerts = [];
    Route.save({key:null}, $scope.newRoute, function() { $scope.newRoute = {}; $scope.list(); },
     function(err) { $scope.alerts = [{msg: err.data.error}]; });
  };

  $scope.save = function(route) {
    $scope.alerts = [];
    Route.save({}, route, function() {$scope.list(); },
     function(err) { $scope.alerts = [{msg: err.data.error}]; });
  };

  $scope.removeBlacklist = function(idx){
    if (confirm('Are you sure you want to delete blacklist entry no. ' + idx)) {
      Blacklist.delete({'index':idx});
      $scope.list();
    }
  };

  $scope.addAggregator = function() {
    $scope.alerts = [];
    Aggregator.save({key:null}, $scope.newAggregator, function() { $scope.newAggregator = {}; $scope.list(); },
     function(err) { $scope.alerts = [{msg: err.data.error}]; });
  };

  $scope.removeAggregator = function(idx){
    if (confirm('Are you sure you want to delete aggregator entry no. ' + idx)) {
      Aggregator.delete({'index':idx});
      $scope.list();
    }
  };

  $scope.removeRoute = function(key){
    if (confirm('Are you sure?')) {
      Route.delete({'key':key});
      $scope.list();
    }
  };

  $scope.removeDestination = function(key, idx){
    if (confirm('Are you sure?')) {
       Destination.delete({'key':key, 'index':idx});
      $scope.list();
    }
  };

  $scope.openRoute = function (idx) {
    var modalInstance = $modal.open({
      templateUrl: 'updateRouteModal.html',
      keyboard: false,
      controller: function ($scope, $modalInstance, route) {
        $scope.route = route;
        $scope.ok = function () {
          $modalInstance.close($scope.route);
        };
        $scope.cancel = function () {
          $modalInstance.dismiss('cancel');
        };
      },
      scope: $scope,
      resolve: {
        route: function () {
          return angular.copy($scope.routes[idx]);
        }
      }
    });

    modalInstance.result.then(function (route) {
      $scope.save(route);
    });
  };

}]);
