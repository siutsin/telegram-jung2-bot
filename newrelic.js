/**
 * New Relic agent configuration.
 *
 * See lib/config.defaults.js in the agent distribution for a more complete
 * description of configuration variables and their potential values.
 */
exports.config = {
  /**
   * Array of application names.
   */
  /*jshint camelcase: false */
  app_name: [process.env.NEWRELIC_APP_NAME],
  /**
   * Your New Relic license key.
   */
  license_key: process.env.NEWRELIC_LICENSE_KEY,
  /*jshint camelcase: true */
  logging: {
    /**
     * Level at which to log. 'trace' is most useful to New Relic when diagnosing
     * issues with the agent, 'info' and higher will impose the least overhead on
     * production applications.
     */
    level: process.env.NEWRELIC_LOGGING_LEVEL
  }
};
