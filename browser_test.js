casper.test.begin('Temp', 3, function suite(test) {
  casper.start('http://localhost:9999/', function() {
    test.assertHttpStatus(200, 'response status code 200');
  });

  casper.thenOpen('http://localhost:9999/zapp', function(response) {
    test.assertHttpStatus(404, 'response status code 404');
    test.assertMatch(response.headers.get('Content-Type'), /json/, 'correct content-type');
  });

  casper.run(function() {
    test.done();
  });
});