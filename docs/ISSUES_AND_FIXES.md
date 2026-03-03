# ConnectPOS – NetSuite: Issues & Fixes (POSIOS-10308)

**Đã thêm trong code:** (1) Restlet set Price Level = "Retail Price" trên Cash Sale header; (2) Go dùng env `NS_DEFAULT_TAX_CODE` khi payload không có tax (vd. `CA_EL CAJON` cho Toor).

## 1. Product price must use **Retail Price**

### Vấn đề
- Đơn CASHSALECS13175: giá sản phẩm trong cart và order details sai.
- POS tính theo **Retail Price** ($1.00) nhưng NetSuite có thể đang dùng giá khác (ví dụ purchase price hoặc price level khác), dẫn đến grand total / total paid sai (ví dụ $5.36 thay vì $10.36).

### Nguyên nhân (trong code hiện tại)
- Restlet `abj_cpos_integration.js` set **`price: "-1"`** trên từng line (dòng 586). Trong NetSuite, `-1` = “use default price”; default có thể không phải Retail (phụ thuộc customer/transaction).
- Transaction (Cash Sale) **không** set **Price Level** = Retail Price ở header, nên default price không được ép về Retail.

### Cách xử lý đã thêm trong repo
1. **Restlet**  
   - Tìm Price Level có name **"Retail Price"** (hoặc tương đương).  
   - Set **Price Level** ở **header** Cash Sale = internal id của Retail Price **trước khi** thêm line.  
   - Giữ nguyên logic set `rate`/`amount` từ payload (POS vẫn gửi đúng Retail); việc set Price Level header đảm bảo khi NetSuite dùng default (ví dụ `price = -1`) thì vẫn là Retail.

2. **POS / connector**  
   - Đảm bảo khi tính giá và gửi sang NetSuite, POS luôn dùng **Retail Price** (không dùng purchase price hay price level khác) cho client này.  
   - Payload gửi sang Restlet phải có `rate` / `priceIncTax` / `amount` tương ứng Retail.

### Cấu hình NetSuite
- Trong Item, tab **Sales / Pricing**: Price Level **Retail Price** (ví dụ cột QTY 0 = 1.00) phải tồn tại và đúng.  
- Tên chính xác của Price Level (vd. `"Retail Price"`) phải khớp với tên dùng trong Restlet khi search (hoặc dùng internal id cố định nếu đã biết).

---

## 2. Apply tax: CA_EL CAJON (8.25%, El Cajon, CA)

### Vấn đề
- Thuế cần áp dụng: **Code: CA_EL CAJON**, Rate: 8.25%, City: EL CAJON (El Cajon, CA 8.25%).  
- Nếu payload không gửi đúng tax code hoặc mapping sai, đơn sẽ không có thuế đúng.

### Cách hoạt động hiện tại
- Go: `createOrderToNetsui.go` / `createOrderToNetsuiUsingNewCashSaleService.go` lấy **tax code** từ `order.TaxLines[0].ExternalID` và gửi vào Restlet dưới dạng `taxCode` cho từng line.  
- Restlet set `taxcode` trên từng line từ `data.items[i].taxCode`.  
- Trong NetSuite, tax code **CA_EL CAJON** đã có (8.25%, El Cajon, CA).

### Cách xử lý
1. **POS / mapping**  
   - Với đơn tại El Cajon, CA: đảm bảo **Tax Line** gửi sang connector có **externalId = "CA_EL CAJON"** (string đúng tên code trong NetSuite).  
   - Connector không đổi tên; giữ nguyên `externalId` để Go map vào `TaxLines[0].ExternalID` → `taxCode` trong payload Restlet.

2. **Fallback trong code (đã thêm)**  
   - Trong `createOrderToNetsui.go` và `createOrderToNetsuiUsingNewCashSaleService.go`: nếu `TaxLines` rỗng hoặc không có `externalId`, sẽ dùng env **`NS_DEFAULT_TAX_CODE`**.  
   - Chỉ dùng fallback nếu business rule rõ: “mặc định cho CA = CA_EL CAJON”.

### Kiểm tra
- Tạo đơn test có 1 line, subtotal $10, tax 8.25% → tax = $0.825, grand total = $10.83.  
- Trong NetSuite, line item phải có Tax Code = CA_EL CAJON và số tiền thuế đúng.

---

## 3. Credentials (đã cập nhật theo comment 23/Feb/26)

- Consumer Key/Secret và Token Id/Secret đã được hiepnc1 cập nhật; cần set đúng trong `.env` / config (ví dụ `NS_CONSUMER_KEY`, `NS_CONSUMER_SECRET`, `NS_ACCESS_TOKEN`, `NS_ACCESS_TOKEN_SECRET`, `NS_ACCOUNT`) cho môi trường Toor Corporation.

---

## 4. Order CASHSALECS13175 – tóm tắt

- **Đúng:** Subtotal $10.00 (10 × $1.00), product “ConnectPOS Test Product 2”.  
- **Sai:** Grand total $5.36, Total paid $5.00, Remaining $0.36.  
- **Nguyên nhân khả dĩ:**  
  - Giá đang bị lấy từ price level khác (vd. 50% off → $0.50/unit) thay vì Retail $1.00.  
  - Hoặc logic tính tổng / payment không đồng bộ với Retail.  
- **Hướng xử lý:** Áp dụng mục 1 (Retail Price) và 2 (tax CA_EL CAJON); đồng bộ config POS + Restlet + NetSuite như trên.
