# DormHub — Railway Deployment (Go + MySQL)

## ก่อนเริ่ม
- สำรองทั้งโฟลเดอร์โปรเจกต์และ Export `DormHub` จาก phpMyAdmin
- โค้ดนี้อิง `main.go` ที่ส่งในแชตล่าสุด บวกหน้า HTML/CSS จากชุด Step 15
- ปรับแล้ว: `PORT` จาก Railway, MySQL จาก env, `SESSION_SECRET`, HTTPS cookies, upload directory บน volume
- หน้าเว็บไซต์และฟังก์ชันเดิมยังอยู่ แต่ต้องทดลองใช้งานบนโฮสต์จริง

## 1. เตรียม GitHub
1. แตก ZIP แล้วอัปโหลดไฟล์ด้านในเป็น root ของ repository ส่วนตัว
2. ห้ามอัปโหลด `.env` หรือเอกสารส่วนบุคคล รวมถึงโฟลเดอร์ `private_uploads` และ `static/uploads`
3. ใช้ `go.mod` และ `go.sum` ในชุดนี้

## 2. Railway
1. เปิด https://railway.com แล้วสร้าง Project และเพิ่ม MySQL service
2. เพิ่ม service จาก GitHub Repo DormHub แล้วเชื่อม MySQL service
3. ใน web service > Variables ให้เพิ่ม reference variables จาก MySQL service ตามรายชื่อต่อไปนี้:
   `MYSQLHOST`, `MYSQLPORT`, `MYSQLUSER`, `MYSQLPASSWORD`, `MYSQLDATABASE`
   *หากชื่อฐานข้อมูลที่ Railway ให้ไม่ใช่ DormHub ไม่เป็นไร เว็บจะใช้ `MYSQLDATABASE` ตามจริง*
4. เพิ่ม `SESSION_SECRET` เป็นข้อความสุ่มยาวอย่างน้อย 32 ตัวอักษร **อย่านำค่านี้ไปใส่ใน GitHub**
5. เพิ่ม `COOKIE_SECURE=true`
6. สร้าง Railway Volume และ mount ที่ `/data`; เพิ่ม `UPLOAD_DIR=/data/uploads` เพื่อให้ไฟล์รูป/เอกสารไม่หายเมื่อรีดีพลอย
7. สร้าง Public Domain แล้วเปิดลิงก์ HTTPS ของเว็บ
8. ตัวโปรแกรมอ่าน `PORT` โดยอัตโนมัติ ไม่มีความจำเป็นต้องกำหนดพอร์ตตายตัว

## 3. Import ฐานข้อมูล
- Railway MySQL มักตั้งชื่อฐานข้อมูลจากตัวแปร `MYSQLDATABASE` (เช่น railway), ไม่จำเป็นต้องชื่อ DormHub
- ไฟล์ `sql/DormHub_Complete_Step15.sql` มี `CREATE DATABASE DormHub; USE DormHub;` สำหรับ **XAMPP เท่านั้น**
- สำหรับฐานข้อมูล Railway ที่ว่าง ใช้ `sql/DormHub_Tables_Railway.sql` ซึ่งสร้าง **เฉพาะ 7 ตารางภายในฐานข้อมูลที่เลือกอยู่** ไม่มี `DROP` หรือ `CREATE DATABASE`
- เชื่อมต่อผ่าน MySQL client หรือเครื่องมือที่ Railway แนะนำ แล้วเลือกฐานข้อมูลของ Railway ก่อน Import
- ถ้าเคย Import แล้ว อย่า Import ซ้ำก่อนตรวจข้อมูล
- โค้ดไม่มีบัญชี Admin เริ่มต้น ให้สมัครบัญชีทดสอบแล้วกำหนด role เฉพาะบัญชีของคุณเองอย่างระมัดระวัง

## 4. การทดสอบ
- เปิดหน้า `/` `/register` `/login` `/search`
- สมัครบัญชี user และทดลองส่งคำขอเช่า
- สมัคร landlord ด้วยเอกสาร **จำลอง** แล้วให้ Admin อนุมัติผ่าน `/admin/landlord-applications`
- เพิ่มหอพักพร้อมรูป JPG/PNG แล้วทดสอบรูปยังอยู่หลัง redeploy

## ข้อควรระวัง
- **ระบบนี้ยังเป็นงานสาธิต** ห้ามใช้เอกสารประจำตัวจริงหรือข้อมูลส่วนตัวจริงบนโฮสต์สาธารณะ
- Login ยังไม่มี rate limiting; ควรเพิ่มก่อนเปิดใช้งานจริง
- การเปิดออนไลน์อาจมีค่าใช้จ่าย และจำเป็นต้องเปิด Railway service + MySQL + Volume พร้อมกัน
- ถ้า deploy build ล้มเหลว ให้ส่ง Build Logs มาเพื่อตรวจ Go toolchain และ dependencies
