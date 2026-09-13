//Работа с данными модальных окон

$(function(){
    $(".editProduct").click(
        function() {
            $("#productId").val($(this).attr('data-id'));
            $("#productNumber").val($(this).attr('data-number'));
            $("#productName").val($(this).attr('data-name'));
            $("#productPrice").val($(this).attr('data-price'));
            $("#productAmount").val($(this).attr('data-amount'));
            $("#productSupplier").val($(this).attr('data-supplier'));
            $("#productStorage").val($(this).attr('data-storage'));
            $("#productEvent").val($(this).attr('data-event'));
    });
});

$(function(){
    $(".deleteProduct").click(
        function() {
            $("#productIdDel").val($(this).attr('data-id'));
            $("#productNumberDel").val($(this).attr('data-number'));
    });
});

$(function(){
    $(".editSupplier").click(
        function() {
            $("#supplierId").val($(this).attr('data-id'));
            $("#supplierName").val($(this).attr('data-name'));
    });
});

$(function(){
    $(".deleteSupplier").click(
        function() {
            $("#supplierIdDel").val($(this).attr('data-id'));
            $("#supplierNameDel").val($(this).attr('data-name'));
    });
});