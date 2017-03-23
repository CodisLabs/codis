/**
 * @license @product.name@ JS v@product.version@ (@product.date@)
 *
 * (c) 2011-2014 Torstein Honsi
 *
 * License: www.highcharts.com/license
 */

/*eslint no-unused-vars: 0 */ // @todo: Not needed in HC5
(function (root, factory) {
    if (typeof module === 'object' && module.exports) {
        module.exports = root.document ? 
        factory(root) :
        function (w) {
            return factory(w);
        };
    } else {
        root.Highcharts = factory();
    }
}(typeof window !== 'undefined' ? window : this, function (w) {
