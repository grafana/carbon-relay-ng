var app = new angular.module("carbon-relay-ng", ["ngResource", "ui.bootstrap"]);

app.controller("MainCtl", ["$scope", "$resource", "$modal", function($scope, $resource, $modal){
  $scope.alerts = [];
  var Route = $resource("/routes/:key", {key: '@key'}, {});

  $scope.validAddress = /^[^:]+\:[0-9]+$/;
  $scope.validPattern = (function() {
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

  $scope.list = function(idx){
    Route.query(function(data){
      $scope.routes = data;
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

  $scope.remove = function(idx){
    if (confirm('Are you sure?')) {
      $scope.routes[idx].$delete();
      $scope.list();
    }
  };

  $scope.open = function (idx) {
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