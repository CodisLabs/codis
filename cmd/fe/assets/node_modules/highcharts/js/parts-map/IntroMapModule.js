/**
 * @license @product.name@ JS v@product.version@ (@product.date@)
 * Highmaps as a plugin for Highcharts 4.1.x or Highstock 2.1.x (x being the patch version of this file)
 *
 * (c) 2011-2014 Torstein Honsi
 *
 * License: www.highcharts.com/license
 */
/* eslint indent: [2, 4] */
(function (factory) {
    if (typeof module === 'object' && module.exports) {
        module.exports = factory;
    } else {
        factory(Highcharts);
    }
}(function (Highcharts) {

